package compile

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/pem"
	"strings"

	"golang.org/x/crypto/ssh"
)

// mppKey is the ed25519 key pair every machine of an MPP cluster shares: the
// management utilities of the engine (gpinitsystem, gpconfig, gpstop) reach
// every host over ssh as gpadmin.
//
// The pair is derived from the run id instead of drawn at random: the RunSpec
// is a pure function of its input, and a compile of the same run must give
// the same files. The key opens only the sshd of the run's own containers,
// which listens on a port reachable from the run's subnets alone and lives as
// long as the run, so deriving it from the (public) run id costs nothing a
// random key would have protected.
//
// doc: https://github.com/stroppy-io/mpp-builds — "Image contract" (/etc/mpp/ssh)
type mppKey struct {
	// Private is the OpenSSH private key file (id_ed25519).
	Private string
	// Public is the OpenSSH public key line (id_ed25519.pub, authorized_keys).
	Public string
}

// mppAuthorized is the authorized_keys line of the cluster: the public key,
// accepted only from the private addresses of the machines (expanded by the
// pipeline once they exist) and loopback, which the engine's utilities may
// use for the host they run on. doc: sshd(8) AUTHORIZED_KEYS FILE FORMAT, from=
func mppAuthorized(public string, machines []string) string {
	return `from="` + strings.Join(append(ipsOf(machines), "127.0.0.1", "::1"), ",") + `" ` + public
}

// newMPPKey derives the key pair of a run.
func newMPPKey(runID string) (mppKey, error) {
	seed := sha256.Sum256([]byte("stroppy-mpp-ssh:" + runID))
	private := ed25519.NewKeyFromSeed(seed[:])
	// An ed25519 private key is its seed followed by its public key.
	public := ed25519.PublicKey(private[ed25519.SeedSize:])
	sshPublic, err := ssh.NewPublicKey(public)
	if err != nil {
		return mppKey{}, err
	}
	// The check integers of the OpenSSH container only detect a wrong
	// passphrase; an unencrypted key repeats one value, drawn from the seed
	// (x/crypto draws it at random, which would make the file differ per call).
	check := binary.BigEndian.Uint32(seed[:4])
	block := struct {
		Check1, Check2 uint32
		KeyType        string
		Public         []byte
		Private        []byte
		Comment        string
		Pad            []byte `ssh:"rest"`
	}{check, check, ssh.KeyAlgoED25519, public, private, "stroppy-mpp", nil}
	// Padding to the cipher block size of "none" (8): 1, 2, 3, …
	encoded := ssh.Marshal(block)
	for i := byte(1); len(encoded)%8 != 0; i++ {
		encoded = append(encoded, i)
	}
	file := ssh.Marshal(struct {
		Cipher, KDF, KDFOptions string
		Keys                    uint32
		Public                  []byte
		Private                 []byte
	}{"none", "none", "", 1, sshPublic.Marshal(), encoded})
	body := append([]byte("openssh-key-v1\x00"), file...)
	return mppKey{
		Private: string(pem.EncodeToMemory(&pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: body})),
		Public:  string(ssh.MarshalAuthorizedKey(sshPublic)),
	}, nil
}
