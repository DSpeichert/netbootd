module github.com/DSpeichert/netbootd

go 1.16

require (
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e
	github.com/fsnotify/fsnotify v1.4.9
	github.com/google/uuid v1.1.4 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20201112113307-4de412bc85d8
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/pin/tftp v2.1.0+incompatible
	github.com/rs/zerolog v1.20.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/viper v1.7.0
	github.com/u-root/u-root v7.0.0+incompatible
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	golang.org/x/sys v0.0.0-20210113000019-eaf3bda374d2
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/pin/tftp => github.com/digitalrebar/tftp v0.0.0-20200914190809-39d58dc90c67
