#!/usr/bin/env python3
"""Install the live-test kubeconfig using one persistent ServiceAccount token."""

import argparse
import base64
import json
from pathlib import Path
import subprocess
import time


TOKEN_SECRET = "stroppy-live-api-token"
SERVICE_ACCOUNT = "stroppy-live"
NAMESPACE = "graphene"


def persistent_token(kubectl):
    """Reuse the controller-managed token; never rotate a populated Secret."""
    account = json.loads(subprocess.check_output(
        kubectl + ["-n", NAMESPACE, "get", "serviceaccount", SERVICE_ACCOUNT, "-o", "json"], text=True))
    account_uid = account["metadata"]["uid"]
    command = kubectl + ["-n", NAMESPACE, "get", "secret", TOKEN_SECRET,
                         "--ignore-not-found", "-o", "json"]
    raw = subprocess.check_output(command, text=True).strip()
    if not raw:
        created = subprocess.run(
            kubectl + ["create", "-f", str(Path(__file__).resolve().parent / "bootstrap/bootstrap-token.yaml")],
            capture_output=True, text=True)
        if created.returncode and "AlreadyExists" not in created.stderr:
            raise RuntimeError("Could not create the persistent ServiceAccount token Secret")
    deadline = time.monotonic() + 30
    while True:
        if raw:
            secret = json.loads(raw)
            annotations = secret["metadata"].get("annotations", {})
            if (secret.get("type") != "kubernetes.io/service-account-token"
                    or annotations.get("kubernetes.io/service-account.name") != SERVICE_ACCOUNT):
                raise RuntimeError("Existing token Secret has an unexpected type or ServiceAccount")
            encoded = secret.get("data", {}).get("token")
            if encoded:
                if annotations.get("kubernetes.io/service-account.uid") != account_uid:
                    raise RuntimeError("Token Secret belongs to a different ServiceAccount UID")
                try:
                    token = base64.b64decode(encoded, validate=True).decode()
                    part = token.split(".")[1]
                    claims = json.loads(base64.urlsafe_b64decode(part + "=" * (-len(part) % 4)))
                except (ValueError, IndexError, UnicodeError):
                    raise RuntimeError("Invalid ServiceAccount token encoding") from None
                if claims.get("sub") != f"system:serviceaccount:{NAMESPACE}:{SERVICE_ACCOUNT}":
                    raise RuntimeError("Unexpected ServiceAccount token subject")
                if "exp" in claims:
                    raise RuntimeError("Expected a persistent token, received an expiring token")
                return token
        if time.monotonic() >= deadline:
            raise RuntimeError("Kubernetes did not populate the ServiceAccount token Secret within 30s")
        time.sleep(1)
        raw = subprocess.check_output(command, text=True).strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--kubeconfig", required=True)
    parser.add_argument("--graphene-config", required=True)
    parser.add_argument("--grant-quota-viewer", action="store_true")
    parser.add_argument("--sync-token-only", "--refresh-token-only", dest="sync_token_only", action="store_true",
                        help="sync the persistent token without applying RBAC or cloud grants; does not rotate it")
    args = parser.parse_args()
    if args.sync_token_only and args.grant_quota_viewer:
        parser.error("--sync-token-only cannot be combined with --grant-quota-viewer")
    kubectl = ["kubectl", "--kubeconfig", args.kubeconfig, "--request-timeout=30s"]
    if not args.sync_token_only:
        subprocess.run(kubectl + ["apply", "-f", str(Path(__file__).resolve().parent / "bootstrap/bootstrap.yaml")], check=True)
    if args.grant_quota_viewer:
        subprocess.run([
            "yc", "resource-manager", "cloud", "add-access-binding", "b1gt6m4l9gfhaobcgb81",
            "--role", "quota-manager.viewer", "--service-account-id", "ajemj32ce93ml1nqk4gb",
        ], check=True)

    # Read the operator's config only to extract the cluster CA. Its identity
    # is never copied into the pipeline's kubeconfig.
    config = json.loads(subprocess.check_output(
        kubectl + ["config", "view", "--minify", "--flatten", "--raw", "-o", "json"], text=True))
    ca = config["clusters"][0]["cluster"]["certificate-authority-data"]
    token = persistent_token(kubectl)
    target = {
        "apiVersion": "v1", "kind": "Config", "current-context": "live",
        "clusters": [{"name": "live", "cluster": {
            "server": "https://kubernetes.default.svc", "certificate-authority-data": ca,
        }}],
        "contexts": [{"name": "live", "context": {"cluster": "live", "user": "stroppy-live"}}],
        "users": [{"name": "stroppy-live", "user": {"token": token}}],
    }
    subprocess.run([
        "graphenectl", "--config", args.graphene_config, "-n", "t-stroppy-live",
        "secret", "set", "kubeconfig",
    ], input=json.dumps(target), text=True, check=True)
    print(f"Installed kubeconfig for {NAMESPACE}/{SERVICE_ACCOUNT}; persistent token Secret: {TOKEN_SECRET}")


if __name__ == "__main__":
    main()
