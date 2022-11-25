#!/bin/sh

# _SCRIPT_VERSION is used to simplify checking local copies of the script.  Bump
# this number every time a significant change is made to this script.
_SCRIPT_VERSION='1'
readonly _SCRIPT_VERSION

verbose="${VERBOSE:-0}"
readonly verbose

# Set verbosity.
if [ "$verbose" -gt '0' ]
then
	set -x
fi

# Set $EXIT_ON_ERROR to zero to see all errors.
if [ "${EXIT_ON_ERROR:-1}" -eq '0' ]
then
	set +e
else
	set -e
fi

# We don't need glob expansions and we want to see errors about unset variables.
set -f -u



# Deferred Helpers

not_found_msg='
looks like a binary not found error.
make sure you have installed the linter binaries using:

	$ make go-tools
'
readonly not_found_msg

not_found() {
	if [ "$?" -eq '127' ]
	then
		# Code 127 is the exit status a shell uses when a command or
		# a file is not found, according to the Bash Hackers wiki.
		#
		# See https://wiki.bash-hackers.org/dict/terms/exit_status.
		echo "$not_found_msg" 1>&2
	fi
}
trap not_found EXIT



# Warnings

go_version="$( "${GO:-go}" version )"
readonly go_version

go_min_version='go1.19.3'
go_version_msg="
warning: your go version (${go_version}) is different from the recommended minimal one (${go_min_version}).
if you have the version installed, please set the GO environment variable.
for example:

	export GO='${go_min_version}'
"
readonly go_min_version go_version_msg

case "$go_version"
in
('go version'*"$go_min_version"*)
	# Go on.
	;;
(*)
	echo "$go_version_msg" 1>&2
	;;
esac



# Simple Analyzers

# blocklist_imports is a simple check against unwanted packages.  The following
# packages are banned:
#
#   *  Packages errors and log are replaced by our own packages in the
#      github.com/AdguardTeam/golibs module.
#
#   *  Package io/ioutil is soft-deprecated.
#
#   *  Package reflect is often an overkill, and for deep comparisons there are
#      much better functions in module github.com/google/go-cmp.  Which is
#      already our indirect dependency and which may or may not enter the stdlib
#      at some point.
#
#      See https://github.com/golang/go/issues/45200.
#
#   *  Package sort is replaced by golang.org/x/exp/slices.
#
#   *  Package unsafe is… unsafe.
#
#   *  Package golang.org/x/net/context has been moved into stdlib.
#
blocklist_imports() {
	git grep\
		-e '[[:space:]]"errors"$'\
		-e '[[:space:]]"io/ioutil"$'\
		-e '[[:space:]]"log"$'\
		-e '[[:space:]]"reflect"$'\
		-e '[[:space:]]"sort"$'\
		-e '[[:space:]]"unsafe"$'\
		-e '[[:space:]]"golang.org/x/net/context"$'\
		-n\
		-- '*.go'\
		| sed -e 's/^\([^[:space:]]\+\)\(.*\)$/\1 blocked import:\2/'\
		|| exit 0
}

# method_const is a simple check against the usage of some raw strings and
# numbers where one should use named constants.
method_const() {
	git grep -F\
		-e '"DELETE"'\
		-e '"GET"'\
		-e '"POST"'\
		-e '"PUT"'\
		-n\
		-- '*.go'\
		| sed -e 's/^\([^[:space:]]\+\)\(.*\)$/\1 http method literal:\2/'\
		|| exit 0
}

# underscores is a simple check against Go filenames with underscores.  Add new
# build tags and OS as you go.  The main goal of this check is to discourage the
# use of filenames like client_manager.go.
underscores() {
	underscore_files="$(
		git ls-files '*_*.go'\
			| grep -F\
			-e '_darwin.go'\
			-e '_generate.go'\
			-e '_linux.go'\
			-e '_test.go'\
			-e '_unix.go'\
			-e '_windows.go'\
			-v\
			| sed -e 's/./\t\0/'
	)"
	readonly underscore_files

	if [ "$underscore_files" != '' ]
	then
		echo 'found file names with underscores:'
		echo "$underscore_files"
	fi
}



# Helpers

# exit_on_output exits with a nonzero exit code if there is anything in the
# command's combined output.
exit_on_output() (
	set +e

	if [ "$VERBOSE" -lt '2' ]
	then
		set +x
	fi

	cmd="$1"
	shift

	output="$( "$cmd" "$@" 2>&1 )"
	exitcode="$?"
	if [ "$exitcode" -ne '0' ]
	then
		echo "'$cmd' failed with code $exitcode"
	fi

	if [ "$output" != '' ]
	then
		if [ "$*" != '' ]
		then
			echo "combined output of linter '$cmd $*':"
		else
			echo "combined output of linter '$cmd':"
		fi

		echo "$output"

		if [ "$exitcode" -eq '0' ]
		then
			exitcode='1'
		fi
	fi

	return "$exitcode"
)



# Checks

exit_on_output blocklist_imports

exit_on_output method_const

exit_on_output underscores

exit_on_output gofumpt --extra -e -l .

# TODO(a.garipov): golint is deprecated, find a suitable replacement.

"$GO" vet ./...

govulncheck ./...

gocyclo --over 10 .

ineffassign ./...

unparam ./...

git ls-files -- 'Makefile' '*.conf' '*.go' '*.mod' '*.sh' '*.yaml' '*.yml'\
	| xargs misspell --error

looppointer ./...

nilness ./...

fieldalignment ./...

exit_on_output shadow --strict ./...

gosec --quiet ./...

errcheck ./...

staticcheck_matrix='
darwin:  GOOS=darwin
linux:   GOOS=linux
windows: GOOS=windows
'
readonly staticcheck_matrix

echo "$staticcheck_matrix" | staticcheck --matrix ./...
