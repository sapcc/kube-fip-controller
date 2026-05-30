// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package frameworks

import "errors"

// ErrFIPNotFound is raised if the FIP cannot be found.
var ErrFIPNotFound = errors.New("FloatingIP not found")

// IsFIPNotFound checks whether the given error is an instance of ErrFIPNotFound.
func IsFIPNotFound(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == ErrFIPNotFound.Error()
}
