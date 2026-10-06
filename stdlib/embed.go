// Package stdlib embeds the vendor library stub sources shipped with stc.
//
// The embed lives here rather than next to the .st files because Go treats
// any directory named "vendor" as a vendor tree: a package under
// stdlib/vendor/ cannot be imported by its full path and ./... skips it.
// Use the per-vendor packages (for example stdlib/beckhoff) instead of
// reading this FS directly.
package stdlib

import "embed"

// VendorFS holds the Beckhoff stub sources under vendor/beckhoff/.
//
//go:embed vendor/beckhoff/*.st
var VendorFS embed.FS
