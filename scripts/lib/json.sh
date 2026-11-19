# Shared by the Claude Code hook scripts. Source it; don't run it.

# json_string prints its standard input as a JSON string literal: control
# characters other than tab and newline are dropped, and backslash, quote,
# tab and newline are escaped.
json_string() {
	local tab=$'\t'
	printf '"'
	LC_ALL=C tr -d '\000-\010\013-\037\177' |
		LC_ALL=C sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' -e "s/${tab}/\\\\t/g" |
		LC_ALL=C awk 'NR > 1 { printf "%s", "\\n" } { printf "%s", $0 }'
	printf '"'
}
