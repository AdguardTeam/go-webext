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

git ls-files -- '*.md' '*.yaml' '*.yml' | xargs misspell --error
