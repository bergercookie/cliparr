package main

import (
	"errors"
	"fmt"
	"strconv"
)

// defaultID is the user and group of the files in the data folder when PUID and
// PGID are not set. It is the account the container image has always used.
const defaultID = 65532

// identity is the user and group that own the data folder and run Cliparr.
type identity struct{ uid, gid int }

// identityFromEnv reads PUID and PGID. PGID defaults to PUID, and PUID to defaultID.
// Cliparr never runs as root, so 0 is refused.
func identityFromEnv(getenv func(string) string) (identity, error) {
	uid, err := parseID("PUID", getenv("PUID"), defaultID)
	if err != nil {
		return identity{}, err
	}
	gid, err := parseID("PGID", getenv("PGID"), uid)
	if err != nil {
		return identity{}, err
	}
	return identity{uid: uid, gid: gid}, nil
}

func parseID(name, value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	id, err := strconv.Atoi(value)
	if err != nil || id < 0 || id > 1<<31-2 {
		return 0, fmt.Errorf("%s %q is not a user or group number: use the output of `id -u` or `id -g`", name, value)
	}
	if id == 0 {
		return 0, errors.New(name + " is 0, which is root: Cliparr does not run as root, use the numbers of your own user from `id -u` and `id -g`")
	}
	return id, nil
}
