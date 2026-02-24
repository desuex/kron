package daemon

import "kron/core/pkg/core"

// CoreVersion proves daemon can link against kron/core in bootstrap phase.
const CoreVersion = core.Version
