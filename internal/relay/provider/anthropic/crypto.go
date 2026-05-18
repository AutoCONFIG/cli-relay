package anthropic

import (
	"crypto/rand"
)

// cryptoRandRead is a package-level variable for testability.
var cryptoRandRead = rand.Read
