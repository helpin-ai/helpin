import unittest

import check_secrets as secrets


class SecretScanTest(unittest.TestCase):
    def names(self, text, path='a.txt'):
        return [name for _, _, name in secrets.scan_text(path, text)]

    def test_flags_provider_tokens(self):
        self.assertEqual(self.names('X: "dp.st.dev.' + 'a' * 40 + '"'), ['Doppler token'])
        self.assertEqual(self.names('k=AKIA' + 'A' * 16), ['AWS access key'])
        self.assertEqual(self.names('-----BEGIN RSA ' + 'PRIVATE KEY-----'), ['Private key'])
        self.assertEqual(self.names('MAXMIND_LICENSE_KEY=' + 'a1' * 10), ['MaxMind license key'])

    def test_flags_remote_database_password_only(self):
        self.assertEqual(self.names('u=postgres://app:' + 'S3cretPassw0rd' + '@pg-prod.corp.net:5432/x'), ['Database URL with password'])
        self.assertEqual(self.names('u=postgres://postgres:postgres@localhost:5432/x'), [])
        self.assertEqual(self.names('u=postgres://user:${DB_PASSWORD}@prod-host/x'), [])

    def test_ignores_placeholders(self):
        self.assertEqual(self.names('MAXMIND_LICENSE_KEY='), [])
        self.assertEqual(self.names('DOPPLER_SECRET: "dp.st.dev.<token>"'), [])

    def test_env_files(self):
        self.assertTrue(secrets.check_path('events-pipeline/rust-capture/.env.kafka'))
        self.assertTrue(secrets.check_path('.env'))
        self.assertFalse(secrets.check_path('server/.env.example'))


if __name__ == '__main__':
    unittest.main()
