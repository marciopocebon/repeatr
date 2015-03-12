package dir

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"hash"
	"os"

	"github.com/spacemonkeygo/errors"
	"github.com/spacemonkeygo/errors/try"
	"polydawn.net/repeatr/def"
	"polydawn.net/repeatr/input"
	"polydawn.net/repeatr/lib/fshash"
)

const Type = "dir"

// interface assertion
var _ input.Input = &Input{}

type Input struct {
	spec          def.Input
	hasherFactory func() hash.Hash
}

func New(spec def.Input) *Input {
	if spec.Type != Type {
		panic(errors.ProgrammerError.New("This input implementation supports definitions of type %q, not %q", Type, spec.Type))
	}
	_, err := os.Stat(spec.URI)
	if os.IsNotExist(err) {
		panic(def.ValidationError.New("Input URI %q must be a directory", spec.URI))
	}
	return &Input{
		spec:          spec,
		hasherFactory: sha512.New384,
	}
}

func (i Input) Apply(destinationRoot string) <-chan error {
	done := make(chan error)
	go func() {
		defer close(done)

		try.Do(func() {
			// walk filesystem, copying and accumulating data for integrity check
			bucket := &fshash.MemoryBucket{}
			err := fshash.FillBucket(i.spec.URI, destinationRoot, bucket, i.hasherFactory)
			if err != nil {
				done <- err
				return
			}

			// hash whole tree
			actualTreeHash, _ := fshash.Hash(bucket, i.hasherFactory)

			// verify total integrity
			expectedTreeHash, err := base64.URLEncoding.DecodeString(i.spec.Hash)
			if !bytes.Equal(actualTreeHash, expectedTreeHash) {
				done <- input.InputHashMismatchError.New("expected hash %q, got %q", i.spec.Hash, base64.URLEncoding.EncodeToString(actualTreeHash))
			}
		}).CatchAll(func(e error) {
			// any errors should be caught and forwarded through a channel for management rather than killing the whole sytem unexpectedly.
			// this could maybe be improved with better error grouping (wrap all errors in a type that indicates origin subsystem and forwards the original as a 'cause' attachement, etc).
			done <- e
		}).Done()
	}()
	return done
}
