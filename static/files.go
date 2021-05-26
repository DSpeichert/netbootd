package static

import "embed"

//go:embed ipxe.efi undionly.kpxe ipxe_arm64.efi
var Files embed.FS
