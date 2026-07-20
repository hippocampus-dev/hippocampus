# Argument Parsing Pattern

How to parse CLI arguments in bash scripts.

## Standard Template

```bash
function usage() {
  cat <<EOS
Usage:
   script-name.sh [options] <args>

Options:
   -h, --help    Show this help
EOS
}

args=()
flags=()
while (( $# )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    -*|--*)
      echo "Unsupported flag $1" 1>&2
      exit 1
      ;;
    *)
      args+=("$1")
      shift
      ;;
  esac
done
```

## Key Points

* `args=()` collects positional arguments
* `flags=()` collects options with values
* `--` marks end of options
* `(( $# ))` checks remaining arguments
* Unknown flags exit with error to stderr
