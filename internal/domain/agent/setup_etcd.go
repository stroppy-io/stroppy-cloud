package agent

// EtcdInstallConfig is the agent payload for etcd installation.
type EtcdInstallConfig struct {
	Version string `json:"version"` // e.g. "3.5.17"
}

// EtcdClusterConfig is the agent payload for etcd cluster setup.
type EtcdClusterConfig struct {
	Name            string `json:"name"`             // this node's etcd name
	InitialCluster  string `json:"initial_cluster"`  // e.g. "etcd0=http://host0:2380,etcd1=http://host1:2380"
	ClientURL       string `json:"client_url"`       // e.g. "http://0.0.0.0:2379"
	PeerURL         string `json:"peer_url"`         // e.g. "http://0.0.0.0:2380"
	AdvertiseClient string `json:"advertise_client"` // e.g. "http://host0:2379"
	AdvertisePeer   string `json:"advertise_peer"`   // e.g. "http://host0:2380"
	State           string `json:"state"`            // "new" or "existing"
}
