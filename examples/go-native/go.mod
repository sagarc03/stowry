module stowry-sign-example

go 1.27

require (
	github.com/sagarc03/stowry v0.3.0
	gopkg.in/yaml.v3 v3.0.1
)

// The example lives in the repository it demonstrates, so it builds against the
// working tree rather than the last release.
replace github.com/sagarc03/stowry => ../..
