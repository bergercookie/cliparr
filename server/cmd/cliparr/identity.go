package main

import (
	"errors"
	"fmt"
	"strconv"
)

// defaultID is the user and group used when only one of PUID and PGID is set. It is
// the account the container image has always used, and the image sets both to it.
const defaultID = 65532

// identity is the user and group that own the data folder and run Cliparr.
type identity struct{ uid, gid int }

// identityFromEnv reads PUID and PGID. It reports set=false when neither is given,
// and then Cliparr keeps the user it was started as: only a container image that
// wants the hand-over sets them. Otherwise PGID defaults to PUID, and PUID to defaultID.
// Cliparr never runs as root, so 0 is refused.
func identityFromEnv(getenv func(string) string) (id identity, set bool, err error) {
	if getenv("PUID") == "" && getenv("PGID") == "" {
		return identity{}, false, nil
	}
	uid, err := parseID("PUID", getenv("PUID"), defaultID)
	if err != nil {
		return identity{}, false, err
	}
	gid, err := parseID("PGID", getenv("PGID"), uid)
	if err != nil {
		return identity{}, false, err
	}
	return identity{uid: uid, gid: gid}, true, nil
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
