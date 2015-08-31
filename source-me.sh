export GOPATH="$(pwd)/external/":"$(pwd)/"
export GOBIN="$(pwd)/bin"
echo "$PATH" | grep "$GOBIN" 1>&2 2>/dev/null
[ $? -ne 0 ] && export PATH="$GOBIN":"$PATH"

echo "GOPATH = $GOPATH"
echo "GOBIN  = $GOBIN"
echo "PATH   = $PATH"
