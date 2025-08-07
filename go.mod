module github.com/DSpeichert/netbootd

go 1.23.0

toolchain go1.24.0

require (
	github.com/Masterminds/sprig/v3 v3.3.0
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/fsnotify/fsnotify v1.9.0
	github.com/gorilla/mux v1.8.1
	github.com/insomniacslk/dhcp v0.0.0-20250417080101-5f8cf70e8c5f
	github.com/pin/tftp v2.1.0+incompatible
	github.com/rs/zerolog v1.34.0
	github.com/spf13/cobra v1.9.1
	github.com/spf13/viper v1.20.1
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701
	golang.org/x/net v0.42.0
	golang.org/x/sys v0.34.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	dario.cat/mergo v1.0.2 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/sagikazarmark/locafero v0.10.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.14.0 // indirect
	github.com/spf13/cast v1.9.2 // indirect
	github.com/spf13/pflag v1.0.7 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/pin/tftp => github.com/digitalrebar/tftp v0.0.0-20200914190809-39d58dc90c67
