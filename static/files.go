package static

import "embed"

//go:embed ipxe.efi undionly.kpxe
var Files embed.FS
