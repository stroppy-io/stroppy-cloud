//go:build ignore

// Test-only controller deployed through execution.additional_containers.
// It can stop/start exactly one labelled database container on its own test VM.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type marker struct {
	ID              int
	Payload, Writer string
}
type container struct {
	ID     string `json:"Id"`
	Labels map[string]string
	State  string
}

func emit(event string, values map[string]any) {
	values["ha_event"] = event
	values["at"] = time.Now().UTC().Format(time.RFC3339Nano)
	_ = json.NewEncoder(os.Stdout).Encode(values)
}

func connect(host string, proxy bool) (*sql.DB, error) {
	if os.Getenv("DB_KIND") == "postgres" {
		port := "5432"
		if proxy {
			port = "5000"
		}
		u := &url.URL{Scheme: "postgres", Host: net.JoinHostPort(host, port), Path: "/postgres", User: url.UserPassword(os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"))}
		u.RawQuery = "sslmode=disable&connect_timeout=3&statement_timeout=3000"
		db, err := sql.Open("pgx", u.String())
		if err == nil {
			db.SetMaxOpenConns(1)
			db.SetMaxIdleConns(0)
		}
		return db, err
	}
	c := mysql.NewConfig()
	c.User, c.Passwd, c.DBName = os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), "stroppy"
	c.Net, c.Addr = "tcp", net.JoinHostPort(host, "3306")
	if proxy {
		c.Addr = net.JoinHostPort(host, "6033")
		c.User, c.Passwd = os.Getenv("PROXY_USER"), os.Getenv("PROXY_PASSWORD")
	} else if os.Getenv("DB_KIND") == "mysql" {
		c.TLSConfig = "skip-verify"
	}
	c.Timeout, c.ReadTimeout, c.WriteTimeout = 3*time.Second, 3*time.Second, 3*time.Second
	db, err := sql.Open("mysql", c.FormatDSN())
	if err == nil {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(0)
	}
	return db, err
}

func retry(ctx context.Context, timeout time.Duration, fn func() error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var last error
	for {
		if last = fn(); last == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("deadline: %w", last)
		case <-time.After(time.Second):
		}
	}
}

func rows(ctx context.Context, db *sql.DB) ([]marker, error) {
	r, err := db.QueryContext(ctx, "SELECT id,payload,writer FROM live_ha_markers ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out := []marker{}
	for r.Next() {
		var m marker
		if err := r.Scan(&m.ID, &m.Payload, &m.Writer); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, r.Err()
}

func compare(ctx context.Context, hosts []string, expected []marker) error {
	for _, host := range hosts {
		db, err := connect(host, false)
		if err != nil {
			return err
		}
		err = retry(ctx, 90*time.Second, func() error {
			got, err := rows(ctx, db)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(got, expected) {
				return fmt.Errorf("rows differ on %s: got %v expected %v", host, got, expected)
			}
			return nil
		})
		db.Close()
		if err != nil {
			return err
		}
	}
	emit("replicas_equal", map[string]any{"hosts": hosts, "rows": expected})
	return nil
}

var docker = &http.Client{Timeout: 25 * time.Second, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", "/var/run/docker.sock")
}}}

func dockerRequest(ctx context.Context, method, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+path, nil)
	if err != nil {
		return nil, err
	}
	r, err := docker.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if r.StatusCode >= 300 {
		return nil, fmt.Errorf("docker %s %s: HTTP %d", method, path, r.StatusCode)
	}
	return b, nil
}

func target(ctx context.Context) (string, error) {
	var found []container
	f, _ := json.Marshal(map[string][]string{"label": {"stroppy-container=" + os.Getenv("TARGET_CONTAINER")}})
	b, err := dockerRequest(ctx, "GET", "/containers/json?filters="+url.QueryEscape(string(f)))
	if err != nil {
		return "", err
	}
	if err = json.Unmarshal(b, &found); err != nil {
		return "", err
	}
	if len(found) != 1 || found[0].State != "running" {
		return "", fmt.Errorf("expected exactly one running target")
	}
	var own []container
	f, _ = json.Marshal(map[string][]string{"label": {"stroppy-container=" + os.Getenv("PROBE_CONTAINER")}})
	b, err = dockerRequest(ctx, "GET", "/containers/json?filters="+url.QueryEscape(string(f)))
	if err != nil {
		return "", err
	}
	if err = json.Unmarshal(b, &own); err != nil {
		return "", err
	}
	if len(own) != 1 || own[0].Labels["stroppy-run"] == "" || own[0].Labels["stroppy-run"] != found[0].Labels["stroppy-run"] {
		return "", fmt.Errorf("target does not belong to this probe run")
	}
	return found[0].ID, nil
}

func execute(ctx context.Context) error {
	proxy, err := connect(os.Getenv("PROXY_HOST"), true)
	if err != nil {
		return err
	}
	defer proxy.Close()
	local, err := connect(os.Getenv("LOCAL_HOST"), false)
	if err != nil {
		return err
	}
	defer local.Close()
	identity := "@@server_uuid"
	if os.Getenv("DB_KIND") == "mariadb" {
		identity = "@@hostname"
	}
	if os.Getenv("DB_KIND") == "postgres" {
		identity = "inet_server_addr()::text"
	}
	var first []marker
	err = retry(ctx, 3*time.Minute, func() error {
		var e error
		first, e = rows(ctx, proxy)
		if e != nil {
			return e
		}
		if len(first) != 1 || first[0].ID != 0 {
			return fmt.Errorf("waiting for workload start marker")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if os.Getenv("WAIT_FOR_LOAD_GATE") == "1" {
		err = retry(ctx, 2*time.Minute, func() error {
			var started int
			if e := proxy.QueryRowContext(ctx, "SELECT started FROM live_ha_control WHERE id=1").Scan(&started); e != nil {
				return e
			}
			if started != 1 {
				return fmt.Errorf("waiting for first successful workload query")
			}
			return nil
		})
		if err != nil {
			return err
		}
		emit("load_gate_open", map[string]any{"host": os.Getenv("LOCAL_HOST")})
	}
	var me string
	if err = local.QueryRowContext(ctx, "SELECT "+identity).Scan(&me); err != nil {
		return err
	}
	if first[0].Writer != me {
		emit("standby_probe", map[string]any{"host": os.Getenv("LOCAL_HOST")})
		return nil
	}
	hosts := strings.Split(os.Getenv("DB_HOSTS"), ",")
	if len(hosts) != 3 {
		return fmt.Errorf("expected three hosts")
	}
	if err = compare(ctx, hosts, first); err != nil {
		return err
	}
	id, err := target(ctx)
	if err != nil {
		return err
	}
	stopped := false
	defer func() {
		if stopped {
			_, e := dockerRequest(context.Background(), "POST", "/containers/"+id+"/start")
			emit("emergency_restart", map[string]any{"success": e == nil})
		}
	}()
	stopped = true // A lost HTTP response does not prove the stop was rejected.
	if _, err = dockerRequest(ctx, "POST", "/containers/"+id+"/stop?t=10"); err != nil {
		return err
	}
	emit("primary_stopped", map[string]any{"host": os.Getenv("LOCAL_HOST"), "container_id": id, "writer": me})
	start := time.Now()
	insert := "INSERT INTO live_ha_markers VALUES (1,'written-while-primary-down'," + identity + ") ON DUPLICATE KEY UPDATE payload=VALUES(payload)"
	if os.Getenv("DB_KIND") == "postgres" {
		insert = "INSERT INTO live_ha_markers VALUES (1,'written-while-primary-down'," + identity + ") ON CONFLICT(id) DO NOTHING"
	}
	err = retry(ctx, 90*time.Second, func() error {
		_, e := proxy.ExecContext(ctx, insert)
		return e
	})
	if err != nil {
		return fmt.Errorf("proxy write during outage: %w", err)
	}
	var after []marker
	err = retry(ctx, 30*time.Second, func() error {
		var e error
		after, e = rows(ctx, proxy)
		if e != nil {
			return e
		}
		if len(after) != 2 {
			return fmt.Errorf("waiting for proxy reader catchup")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if after[1].Writer == me {
		return fmt.Errorf("proxy write did not change serving node")
	}
	emit("proxy_write_recovered", map[string]any{"elapsed_seconds": time.Since(start).Seconds(), "new_writer": after[1].Writer, "rows": after})
	remaining := []string{}
	pgPrimaries := 0
	for _, h := range hosts {
		if h != os.Getenv("LOCAL_HOST") {
			remaining = append(remaining, h)
		}
	}
	if len(remaining) != 2 {
		return fmt.Errorf("local host missing from input hosts")
	}
	if err = compare(ctx, remaining, after); err != nil {
		return err
	}
	if _, err = dockerRequest(ctx, "POST", "/containers/"+id+"/start"); err != nil {
		return err
	}
	stopped = false
	emit("primary_restarted", map[string]any{"container_id": id})
	if err = compare(ctx, hosts, after); err != nil {
		return err
	}
	for _, h := range hosts {
		db, e := connect(h, false)
		if e != nil {
			return e
		}
		if os.Getenv("DB_KIND") == "postgres" {
			var replica bool
			err = db.QueryRowContext(ctx, "SELECT pg_is_in_recovery()").Scan(&replica)
			db.Close()
			if err != nil {
				return err
			}
			if !replica {
				pgPrimaries++
			}
			continue
		}
		err = retry(ctx, 90*time.Second, func() error {
			var count int
			q := "SELECT COUNT(*) FROM performance_schema.replication_group_members WHERE MEMBER_STATE='ONLINE'"
			if os.Getenv("DB_KIND") == "mariadb" {
				q = "SELECT CAST(VARIABLE_VALUE AS UNSIGNED) FROM information_schema.GLOBAL_STATUS WHERE VARIABLE_NAME='WSREP_CLUSTER_SIZE'"
			}
			if e := db.QueryRowContext(ctx, q).Scan(&count); e != nil {
				return e
			}
			if count != 3 {
				return fmt.Errorf("cluster has %d ready members", count)
			}
			return nil
		})
		db.Close()
		if err != nil {
			return err
		}
	}
	if os.Getenv("DB_KIND") == "postgres" && pgPrimaries != 1 {
		return fmt.Errorf("expected one PostgreSQL primary, got %d", pgPrimaries)
	}
	if _, err = proxy.ExecContext(ctx, "UPDATE live_ha_control SET done=1 WHERE id=1"); err != nil {
		return err
	}
	emit("verified", map[string]any{"replicas": len(hosts), "rows": after, "primary_changed": true, "primary_rejoined": true})
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "health" {
		if _, err := os.Stat("/tmp/ha-probe-ready"); err != nil {
			os.Exit(1)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := os.WriteFile("/tmp/ha-probe-ready", []byte("ready"), 0600); err != nil {
		panic(err)
	}
	err := execute(ctx)
	if err != nil {
		emit("failed", map[string]any{"error": err.Error()})
	}
	<-ctx.Done()
}
