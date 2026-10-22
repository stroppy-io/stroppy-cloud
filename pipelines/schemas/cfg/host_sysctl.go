package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// HostSysctl is cfg.host.sysctl@1 — kernel tunables applied to every machine
// of a run before the database starts. Rendered as an /etc/sysctl.d drop-in.
//
// Every key is checked against the mainline kernel sysctl documentation:
//   - vm.*     doc: https://www.kernel.org/doc/html/latest/admin-guide/sysctl/vm.html
//   - kernel.* doc: https://www.kernel.org/doc/html/latest/admin-guide/sysctl/kernel.html
//   - fs.*     doc: https://www.kernel.org/doc/html/latest/admin-guide/sysctl/fs.html
//   - net.*    doc: https://www.kernel.org/doc/html/latest/admin-guide/sysctl/net.html
//     and      https://www.kernel.org/doc/html/latest/networking/ip-sysctl.html
//
// transparent_hugepage is NOT a sysctl (it lives in sysfs); it is carried here
// because it belongs to the same host-prep step and is rendered as a separate
// sysfs write line.
func HostSysctl() *schemapb.Schema {
	return schemapb.NewSchema(ids.Cfg("host.sysctl", 1)).
		Descr("Kernel tunables (sysctl.d drop-in) applied to every machine of a run.").
		Strict().Coerce().
		Fields(
			// doc: admin-guide/sysctl/vm.html#swappiness — 0..200 since 5.8.
			schemapb.Int64("vm_swappiness").Title("vm.swappiness").Group("Memory").
				Desc("Swap aggressiveness, 0..200 (kernel default 60). Benchmarks keep it at 1: swapping falsifies latency.").
				Gte(0).Lte(200).Default(1),
			// doc: admin-guide/sysctl/vm.html#overcommit-memory
			schemapb.Choice("vm_overcommit_memory").Title("vm.overcommit_memory").Group("Memory").
				Desc("0 = heuristic (kernel default), 1 = always overcommit, 2 = strict (CommitLimit = swap + ratio% RAM).").
				Opt(schemapb.Int64V(0), "0 — heuristic").
				Opt(schemapb.Int64V(1), "1 — always overcommit").
				Opt(schemapb.Int64V(2), "2 — strict").
				Default(schemapb.Int64V(0)),
			// doc: admin-guide/sysctl/vm.html#overcommit-ratio
			schemapb.Int64("vm_overcommit_ratio").Title("vm.overcommit_ratio").Group("Memory").
				Desc("Percent of physical RAM added to swap to form CommitLimit; only read when vm.overcommit_memory = 2.").
				Unit("%").Gte(0).Lte(100).Default(50),
			// doc: admin-guide/sysctl/vm.html#dirty-ratio
			schemapb.Int64("vm_dirty_ratio").Title("vm.dirty_ratio").Group("Writeback").
				Desc("Percent of available memory of dirty pages at which a writing process itself starts writeback (kernel default 20).").
				Unit("%").Gte(0).Lte(100).Default(20),
			// doc: admin-guide/sysctl/vm.html#dirty-background-ratio
			schemapb.Int64("vm_dirty_background_ratio").Title("vm.dirty_background_ratio").Group("Writeback").
				Desc("Percent of available memory of dirty pages at which the background flusher starts (kernel default 10).").
				Unit("%").Gte(0).Lte(100).Default(10),
			// doc: admin-guide/sysctl/vm.html#max-map-count
			schemapb.Int64("vm_max_map_count").Title("vm.max_map_count").Group("Memory").
				Desc("Maximum number of memory-map areas per process (kernel default 65530).").
				Gte(65530).Lte(1048576).Default(262144),
			// doc: admin-guide/sysctl/vm.html#nr-hugepages
			schemapb.Int64("vm_nr_hugepages").Title("vm.nr_hugepages").Group("Hugepages").
				Desc("Number of pre-allocated persistent huge pages; 0 leaves huge pages unused (kernel default 0).").
				Gte(0).Lte(1048576).Default(0),
			// doc: admin-guide/sysctl/kernel.html#shmmax (bytes, one segment)
			schemapb.Int64("kernel_shmmax").Title("kernel.shmmax").Group("SysV IPC").
				Desc("Maximum size of one System V shared-memory segment, bytes. Unset leaves the kernel default (ULONG_MAX on 64-bit).").
				Unit("bytes").Gte(1048576).Nullable(),
			// doc: admin-guide/sysctl/kernel.html#shmall (pages)
			schemapb.Int64("kernel_shmall").Title("kernel.shmall").Group("SysV IPC").
				Desc("Total System V shared memory the system may allocate, in PAGES (not bytes). Unset leaves the kernel default.").
				Unit("pages").Gte(1024).Nullable(),
			// doc: admin-guide/sysctl/kernel.html#sem — "SEMMSL SEMMNS SEMOPM SEMMNI"
			schemapb.Str("kernel_sem").Title("kernel.sem").Group("SysV IPC").
				Desc(`System V semaphore limits, four integers "SEMMSL SEMMNS SEMOPM SEMMNI".`).
				Pattern(`^[0-9]+ [0-9]+ [0-9]+ [0-9]+$`).Default("250 32000 100 128").
				Examples(schemapb.StrV("250 32000 100 128")),
			// doc: admin-guide/sysctl/fs.html#file-max
			schemapb.Int64("fs_file_max").Title("fs.file-max").Group("Filesystem").
				Desc("System-wide maximum number of open file handles.").
				Gte(8192).Lte(100000000).Default(2097152),
			// doc: admin-guide/sysctl/fs.html#aio-max-nr
			schemapb.Int64("fs_aio_max_nr").Title("fs.aio-max-nr").Group("Filesystem").
				Desc("Maximum number of outstanding async I/O requests system-wide (kernel default 65536); raised for io_uring/AIO storage engines.").
				Gte(65536).Lte(16777216).Default(1048576),
			// doc: admin-guide/sysctl/net.html#somaxconn
			schemapb.Int64("net_core_somaxconn").Title("net.core.somaxconn").Group("Network").
				Desc("Maximum accept-queue length per listening socket (kernel default 4096 since 5.4).").
				Gte(128).Lte(1048576).Default(4096),
			// doc: admin-guide/sysctl/net.html#rmem-max
			schemapb.Int64("net_core_rmem_max").Title("net.core.rmem_max").Group("Network").
				Desc("Maximum receive socket buffer a program may request with SO_RCVBUF, bytes.").
				Unit("bytes").Gte(65536).Lte(1073741824).Default(16777216),
			// doc: admin-guide/sysctl/net.html#wmem-max
			schemapb.Int64("net_core_wmem_max").Title("net.core.wmem_max").Group("Network").
				Desc("Maximum send socket buffer a program may request with SO_SNDBUF, bytes.").
				Unit("bytes").Gte(65536).Lte(1073741824).Default(16777216),
			// doc: networking/ip-sysctl.html tcp_keepalive_time
			schemapb.Int64("net_ipv4_tcp_keepalive_time").Title("net.ipv4.tcp_keepalive_time").Group("Network").
				Desc("Seconds an idle TCP connection waits before the first keepalive probe (kernel default 7200).").
				Unit("s").Gte(30).Lte(7200).Default(300),
			// doc: networking/ip-sysctl.html tcp_keepalive_intvl
			schemapb.Int64("net_ipv4_tcp_keepalive_intvl").Title("net.ipv4.tcp_keepalive_intvl").Group("Network").
				Desc("Seconds between keepalive probes (kernel default 75).").
				Unit("s").Gte(1).Lte(300).Default(30),
			// doc: networking/ip-sysctl.html tcp_keepalive_probes
			schemapb.Int64("net_ipv4_tcp_keepalive_probes").Title("net.ipv4.tcp_keepalive_probes").Group("Network").
				Desc("Unacknowledged keepalive probes before the connection is dropped (kernel default 9).").
				Gte(1).Lte(30).Default(5),
			// doc: networking/ip-sysctl.html ip_local_port_range
			schemapb.Str("net_ipv4_ip_local_port_range").Title("net.ipv4.ip_local_port_range").Group("Network").
				Desc(`Ephemeral port range for outgoing connections, "<first> <last>"; empty keeps the kernel's "32768 60999". A range reaching below the databases' service ports lets outgoing connections take them.`).
				Pattern(`^[0-9]{1,5} [0-9]{1,5}$`).Nullable(),
			// doc: admin-guide/mm/transhuge.html — sysfs, not sysctl.
			schemapb.Choice("transparent_hugepage").Title("Transparent hugepages").Group("Hugepages").
				Desc("/sys/kernel/mm/transparent_hugepage/enabled. Databases with their own buffer pools want never or madvise.").
				Opt(schemapb.StrV("never"), "never").
				Opt(schemapb.StrV("madvise"), "madvise").
				Opt(schemapb.StrV("always"), "always").
				Default(schemapb.StrV("never")),

			schemapb.MapOf("custom", schemapb.Str("value").MaxLen(256)).
				Title("Extra sysctl keys").Group("Custom").
				Desc("Any further sysctl key → value, appended verbatim after the known keys.").
				MaxEntries(64).
				Rules(schemapb.Rule(
					`this.all(k, k.matches("^[a-z0-9_]+([.][a-z0-9_-]+)+$"))`,
					"sysctl keys look like net.ipv4.tcp_fastopen").ID("custom-key-shape")),

			// The custom map has no fixed key set, so it cannot be printed by
			// a logic-less template: build the lines here and print the block.
			schemapb.Computed("custom_lines",
				`("custom" in root) ? root.custom.map(k, k + " = " + string(root.custom[k])).join("\n") : ""`).
				Result(schemapb.ResultString).Group("Custom").
				Title("Rendered custom lines").Desc("Derived: the custom map as sysctl.conf lines."),
		).
		Rules(
			schemapb.Rule("int(root.vm_dirty_background_ratio) <= int(root.vm_dirty_ratio)",
				"vm.dirty_background_ratio must be <= vm.dirty_ratio").ID("dirty-bg-le-dirty"),
		).
		Template("conf", `# managed by stroppy-cloud — cfg.host.sysctl@1
vm.swappiness = {{{values.vm_swappiness}}}
vm.overcommit_memory = {{{values.vm_overcommit_memory}}}
vm.overcommit_ratio = {{{values.vm_overcommit_ratio}}}
vm.dirty_ratio = {{{values.vm_dirty_ratio}}}
vm.dirty_background_ratio = {{{values.vm_dirty_background_ratio}}}
vm.max_map_count = {{{values.vm_max_map_count}}}
vm.nr_hugepages = {{{values.vm_nr_hugepages}}}
{{#values.kernel_shmmax}}kernel.shmmax = {{{values.kernel_shmmax}}}
{{/values.kernel_shmmax}}{{#values.kernel_shmall}}kernel.shmall = {{{values.kernel_shmall}}}
{{/values.kernel_shmall}}kernel.sem = {{{values.kernel_sem}}}
fs.file-max = {{{values.fs_file_max}}}
fs.aio-max-nr = {{{values.fs_aio_max_nr}}}
net.core.somaxconn = {{{values.net_core_somaxconn}}}
net.core.rmem_max = {{{values.net_core_rmem_max}}}
net.core.wmem_max = {{{values.net_core_wmem_max}}}
net.ipv4.tcp_keepalive_time = {{{values.net_ipv4_tcp_keepalive_time}}}
net.ipv4.tcp_keepalive_intvl = {{{values.net_ipv4_tcp_keepalive_intvl}}}
net.ipv4.tcp_keepalive_probes = {{{values.net_ipv4_tcp_keepalive_probes}}}
{{#values.net_ipv4_ip_local_port_range}}net.ipv4.ip_local_port_range = {{{values.net_ipv4_ip_local_port_range}}}
{{/values.net_ipv4_ip_local_port_range}}{{#values.custom_lines}}{{{values.custom_lines}}}
{{/values.custom_lines}}
# not a sysctl — written to sysfs by the host-prep step:
# echo {{{values.transparent_hugepage}}} > /sys/kernel/mm/transparent_hugepage/enabled
`).
		MustBuild()
}
