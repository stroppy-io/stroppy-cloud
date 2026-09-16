import base64
import json
import subprocess
import unittest
from unittest.mock import patch

import bootstrap


class PersistentTokenTests(unittest.TestCase):
    def secret(self, *, exp=None, subject=None, uid="account-uid"):
        claims = {"sub": subject or "system:serviceaccount:graphene:stroppy-live"}
        if exp is not None:
            claims["exp"] = exp
        payload = base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip("=")
        token = "header." + payload + ".signature"
        return token, {"type": "kubernetes.io/service-account-token", "metadata": {"annotations": {
            "kubernetes.io/service-account.name": "stroppy-live",
            "kubernetes.io/service-account.uid": uid,
        }}, "data": {"token": base64.b64encode(token.encode()).decode()}}

    def outputs(self, secret):
        return [json.dumps({"metadata": {"uid": "account-uid"}}), json.dumps(secret)]

    def test_repeated_bootstrap_reuses_token_without_mutating_secret(self):
        token, secret = self.secret()
        with patch("bootstrap.subprocess.check_output", side_effect=self.outputs(secret) * 2), patch("bootstrap.subprocess.run") as mutate:
            self.assertEqual(bootstrap.persistent_token(["kubectl"]), token)
            self.assertEqual(bootstrap.persistent_token(["kubectl"]), token)
            mutate.assert_not_called()

    def test_missing_secret_is_created_and_controller_population_is_waited_for(self):
        token, secret = self.secret()
        pending = {**secret, "data": {}}
        responses = [self.outputs(secret)[0], "", json.dumps(pending), json.dumps(secret)]
        with patch("bootstrap.subprocess.check_output", side_effect=responses), patch("bootstrap.subprocess.run", return_value=subprocess.CompletedProcess([], 0)) as create, patch("bootstrap.time.sleep"):
            self.assertEqual(bootstrap.persistent_token(["kubectl"]), token)
            create.assert_called_once()
            self.assertIn("bootstrap-token.yaml", create.call_args.args[0][-1])

    def test_expiring_token_is_rejected_without_rotation(self):
        _, secret = self.secret(exp=9999999999)
        with patch("bootstrap.subprocess.check_output", side_effect=self.outputs(secret)), patch("bootstrap.subprocess.run") as mutate:
            with self.assertRaisesRegex(RuntimeError, "expiring token"):
                bootstrap.persistent_token(["kubectl"])
            mutate.assert_not_called()

    def test_other_service_account_token_is_rejected(self):
        _, secret = self.secret(subject="system:serviceaccount:graphene:other")
        with patch("bootstrap.subprocess.check_output", side_effect=self.outputs(secret)):
            with self.assertRaisesRegex(RuntimeError, "subject"):
                bootstrap.persistent_token(["kubectl"])

    def test_recreated_service_account_is_not_silently_rebound(self):
        _, secret = self.secret(uid="old-uid")
        with patch("bootstrap.subprocess.check_output", side_effect=self.outputs(secret)):
            with self.assertRaisesRegex(RuntimeError, "UID"):
                bootstrap.persistent_token(["kubectl"])


if __name__ == "__main__":
    unittest.main()
