module github.com/DSpeichert/netbootd

go 1.16

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf
	github.com/fsnotify/fsnotify v1.4.9
	github.com/google/uuid v1.2.0 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20210621130208-1cac67f12b1e
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/pin/tftp v2.1.0+incompatible
	github.com/rs/zerolog v1.23.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.8.1
	github.com/stretchr/objx v0.1.1 // indirect
	github.com/u-root/uio v0.0.0-20210528151154-e40b768296a7
	golang.org/x/crypto v0.0.0-20210616213533-5ff15b29337e // indirect
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/pin/tftp => github.com/digitalrebar/tftp v0.0.0-20200914190809-39d58dc90c67
