// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package insamlare

import "time"

var nowUTC = func() time.Time {
	return time.Now().UTC()
}
