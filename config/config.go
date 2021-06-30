package config

type Config struct {
	Api struct {
		Authorization      string
		TLSPrivateKeyPath  string
		TLSCertificatePath string
	}
}
