// Package opcua serves a TF6100-shaped OPC UA address space for a running
// Structured Text project.
//
// The server is built on github.com/awcullen/opcua and publishes symbols
// from an abstract symbol model: the SymbolNode tree describes what exists
// and the NodeSource supplies live values. The package deliberately does
// not depend on pkg/symtree, pkg/interp or pkg/vendor; adapters for those
// live elsewhere so this package can be built and tested in isolation.
package opcua
