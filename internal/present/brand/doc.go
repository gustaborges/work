// Package brand assembles the static WORK wordmark shown by bare interactive
// `work` and as the `work --help` header (contracts/brand.md). Render is a pure
// function of (width, colour profile, colour enabled): a multi-line terminal-art
// wordmark with a per-column primary→secondary gradient when the terminal is
// wide and true-colour capable, the same art without colour when colour is off,
// and a compact plain WORK when the terminal is too narrow. The art is a static
// embedded constant; there is no runtime figlet dependency.
package brand
