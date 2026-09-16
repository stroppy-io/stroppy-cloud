#!/usr/bin/env python3
"""Read-only YDB viewer checks for every member of a compiled test stand."""
import argparse
import json
import re
import urllib.parse
import urllib.request


def inspect(endpoint, version, storage_nodes, compute_nodes, zones, database="/Root/stroppy"):
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    def read(path):
        with opener.open(endpoint.rstrip("/") + "/viewer/json/" + path, timeout=20) as response:
            return json.load(response)

    nodes = read("nodes")
    cluster = read("cluster")
    storage = read("storage")
    tenants = read("tenantinfo?" + urllib.parse.urlencode({"database": database}))
    expected = storage_nodes + compute_nodes
    assert int(nodes["TotalNodes"]) == int(nodes["FoundNodes"]) == expected, nodes
    assert cluster["NodesTotal"] == cluster["NodesAlive"] == expected, cluster
    assert cluster["Overall"] == "Green", cluster
    members = []
    for node in nodes["Nodes"]:
        state = node["SystemState"]
        assert state["SystemState"] == "Green", state
        release = re.match(r"^(?:stable-)?(\d+)[.-](\d+)[.-]", state["Version"])
        assert release and ".".join(release.groups()) == version, state["Version"]
        members.append({
            "id": node["NodeId"], "version": state["Version"],
            "state": state["SystemState"], "roles": state["Roles"],
            "zone": state["Location"]["DataCenter"],
        })
    assert len(members) == len({x["id"] for x in members}) == expected
    storage_ids = {x["id"] for x in members if "Storage" in x["roles"]}
    compute_ids = {x["id"] for x in members if "Tenant" in x["roles"]}
    assert len(storage_ids) == storage_nodes and len(compute_ids) == compute_nodes
    assert sorted({x["zone"] for x in members}) == sorted(zones)
    tenant = next(x for x in tenants["TenantInfo"] if x["Name"] == database)
    assert tenant["State"] == "RUNNING" and tenant["Overall"] == "Green", tenant
    assert set(tenant["NodeIds"]) == compute_ids and tenant["AliveNodes"] == compute_nodes
    erasure = "none" if storage_nodes == 1 else "mirror-3-dc"
    groups = []
    for pool in storage["StoragePools"]:
        assert pool["Overall"] == "Green", pool["Name"]
        for group in pool["Groups"]:
            assert group["Overall"] == "Green" and group["ErasureSpecies"] == erasure
            assert set(group["VDiskNodeIds"]) == storage_ids, group
            for disk in group["VDisks"]:
                assert disk["VDiskState"] == "OK" and disk["Replicated"] is True, disk
                assert disk["PDisk"]["State"] == "Normal" and not disk.get("HasUnreadableBlobs", False), disk
            groups.append({"pool": pool["Name"], "id": group["GroupID"], "erasure": erasure,
                           "state": group["Overall"], "vdisk_nodes": group["VDiskNodeIds"]})
    assert any(x["pool"] == database + ":ssd" for x in groups), groups
    return {"probe": "ydb", "status": "passed", "members": members,
            "database": database, "tenant_state": tenant["State"], "storage_groups": groups}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--endpoint", default="http://127.0.0.1:8765")
    parser.add_argument("--version", required=True)
    parser.add_argument("--storage-nodes", type=int, required=True)
    parser.add_argument("--compute-nodes", type=int, required=True)
    parser.add_argument("--zones", required=True, help="Comma-separated YC zones")
    args = parser.parse_args()
    print(json.dumps(inspect(args.endpoint, args.version, args.storage_nodes, args.compute_nodes, args.zones.split(","))))


if __name__ == "__main__":
    main()
