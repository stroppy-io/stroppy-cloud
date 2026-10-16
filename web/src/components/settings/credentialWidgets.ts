import { secretFileWidget } from '@components/schema/SecretFileInput'

// Credential fields that come as a file: the Yandex Cloud authorized key (key.json).
export const credentialWidgets = {
  sa_key_json: secretFileWidget('.json,application/json'),
}
