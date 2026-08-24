package engine

import (
	"regexp"
	"strconv"
	"strings"
)

var leadingNumber = regexp.MustCompile(`(\d+)`)

// parseMajor extracts the major version from a version string such as
// "8.0.46", "16.15" or "5.5.5-10.11.8-MariaDB" (MariaDB reports the real
// version after the compatibility prefix).
func parseMajor(version string) int {
	v := strings.TrimSpace(version)
	if strings.Contains(strings.ToLower(v), "mariadb") {
		if i := strings.Index(v, "-"); i >= 0 {
			v = v[i+1:]
		}
	}
	m := leadingNumber.FindString(v)
	if m == "" {
		return 0
	}
	n, _ := strconv.Atoi(m)
	return n
}

// mysqlVersionCompat compares source and target MySQL server versions.
// It returns an optional warning and an optional fatal compatibility error.
func mysqlVersionCompat(source, target string) (warning, fatal string) {
	srcMajor := parseMajor(source)
	tgtMajor := parseMajor(target)
	if srcMajor == 0 || tgtMajor == 0 {
		return "could not parse MySQL server versions", ""
	}
	if srcMajor == tgtMajor {
		return "", ""
	}
	if tgtMajor < srcMajor {
		return "", "target MySQL version " + target + " is older than source " + source +
			"; restoring a newer dump into an older server is not supported"
	}
	return "target MySQL version " + target + " is newer than source " + source +
		"; verify feature compatibility before relying on the result", ""
}

// pgVersionCompat validates the pg_dump client and source/target servers.
func pgVersionCompat(clientMajor, sourceMajor, targetMajor int) (warning, fatal string) {
	if clientMajor == 0 || sourceMajor == 0 {
		return "could not determine PostgreSQL versions", ""
	}
	if clientMajor < sourceMajor {
		return "", "pg_dump client version " + strconv.Itoa(clientMajor) +
			" cannot dump source PostgreSQL " + strconv.Itoa(sourceMajor) +
			" (client must be >= source major version)"
	}
	if targetMajor > 0 && targetMajor < sourceMajor {
		return "", "target PostgreSQL " + strconv.Itoa(targetMajor) +
			" is older than source " + strconv.Itoa(sourceMajor) +
			"; restoring into an older server is not supported"
	}
	if targetMajor > 0 && targetMajor != sourceMajor {
		return "source and target PostgreSQL major versions differ (" +
			strconv.Itoa(sourceMajor) + " vs " + strconv.Itoa(targetMajor) + ")", ""
	}
	return "", ""
}
