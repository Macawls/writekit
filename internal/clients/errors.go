package clients

import "errors"

var ErrNPXMissing = errors.New("this client needs Node.js with npx on your PATH — install Node from nodejs.org, then try again")

var errNPXMissing = ErrNPXMissing
