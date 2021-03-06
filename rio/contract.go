package rio

import (
	"github.com/inconshreveable/log15"
	"github.com/spacemonkeygo/errors"

	"go.polydawn.net/repeatr/api/def"
	"go.polydawn.net/repeatr/rio/filter"
)

/*
	String describing a type of data transmat.  These are the keys used in plugin registration,
	and are used to dispatch input/output configurations to their appropriate drivers.

	TransmatKind labels must be devoid of slashes and other special characters.
*/
type TransmatKind string

type SiloURI string

type CommitID string

type Arena interface {
	Path() string
	Hash() CommitID
	Teardown()
}

type Transmat interface {
	/*
		`Materialize` causes data to exist, and returns an Arena which
		describes where the requested data can be seen on a local posix filesystem.

		In the bigger picture: Transmat.Materialize causes data to exist;
		Placers and Assemblers get several data sets to resemble a single
		filesystem; Executors then plop that whole thing into a sandbox.

		This may be a relatively long-running operation, and using a goroutine
		to parallelize materialize operations is advised.
		(See `executor/util/provision.go`.)

		Integrity: The Transmat implementation is responsible for ensuring that the content
		of this Arena matches the hash described by the `CommitID` parameter.

		Sharing and Mutability: The Transmat implementation is allowed to assume
		no other process will mutate the filesystem.  Thus, the Transmat may
		return an Arena that refers to a filesystem shared with other Arenas.
		**It is the caller's responsibility not to modify this filesystem, and undefined
		behavior may result from the whole system if this contract is violated.**
		The caller should use `io/placer` systems to isolate filesystems
		before operating on them or exposing them to other systems.

		Locality: Implementation-specific.  Many Transmats will actually produce
		data onto a plain local filesystem (and thus, behaviors of your host
		filesystem may leak through!  noatime, etc); others may use FUSE or
		other custom mounting sytems.
		The data need not all be available on the local machine by the time
		the Arena is returned; it may or may not be cached locally after Arena teardown.
		The behavior of fsync is not guaranteed.
		The only requirement is that it look roughly like a posix filesystem.

		Composability: Transmats may delegate based on the `TransmatKind`
		parameter.  See `rio.DispatchingTransmat` for an example of this.

	*/
	Materialize(kind TransmatKind, dataHash CommitID, siloURIs []SiloURI, log log15.Logger, options ...MaterializerConfigurer) Arena

	Scan(kind TransmatKind, subjectPath string, siloURIs []SiloURI, log log15.Logger, options ...MaterializerConfigurer) CommitID

	/*
		Returns a list of all active Arenas managed by this Transmat.

		This isn't often used, since most work can be done through the idempotent
		materializer method, but it *is* critical for having the ability to do
		cleanup on a system that suffered an unexpected halt.
	*/
	//Arenas() []Arena
	// REMOVED: this doesn't seem to be very useful in general.  just for caching transmats.
	// subject to review, but a use case needs to be demonstrated, because this makes a bunch of things stateful that have no reason to be.
}

/*
	Factory function interface for Transmats.  Plugins must implement this.

	Takes a workdir path, and that's it.  This is may be expected to double-time
	it both as a fresh starter, and be able to recognize and attempt to
	recover ruins from a prior run.
*/
type TransmatFactory func(workPath string) Transmat

type MaterializerOptions struct {
	// TODO play more with how this pattern works (or doesn't) with embedding n stuff.
	// I'd be nice to have just one ProgressReporter configurator for both input and output systems, for example.

	ProgressReporter   chan<- float32
	AcceptHashMismatch bool

	FilterSet filter.FilterSet
}

type MaterializerConfigurer func(*MaterializerOptions)

// not technically necessary as a type, but having this MaterializerFactoryConfigurer symbol exported means godoc groups things helpfully,

func ProgressReporter(rep chan<- float32) MaterializerConfigurer {
	return func(opts *MaterializerOptions) {
		opts.ProgressReporter = rep
	}
}

var AcceptHashMismatch = func(opts *MaterializerOptions) {
	opts.AcceptHashMismatch = true
}

func UseFilter(filt filter.Filter) MaterializerConfigurer {
	return func(opts *MaterializerOptions) {
		opts.FilterSet = opts.FilterSet.Put(filt)
	}
}

func ConvertFilterConfig(conf def.Filters) []MaterializerConfigurer {
	filterOptions := make([]MaterializerConfigurer, 0, 3)
	switch conf.UidMode {
	case def.FilterKeep: // easy, just no filter.
	case def.FilterUse:
		f := filter.UidFilter{conf.Uid}
		filterOptions = append(filterOptions, UseFilter(f))
	default:
		panic(errors.ProgrammerError.New("unhandled filter mode %v", conf.UidMode))
	}
	switch conf.GidMode {
	case def.FilterKeep: // easy, just no filter.
	case def.FilterUse:
		f := filter.GidFilter{conf.Gid}
		filterOptions = append(filterOptions, UseFilter(f))
	default:
		panic(errors.ProgrammerError.New("unhandled filter mode %v", conf.GidMode))
	}
	switch conf.MtimeMode {
	case def.FilterKeep: // easy, just no filter.
	case def.FilterUse:
		f := filter.MtimeFilter{conf.Mtime}
		filterOptions = append(filterOptions, UseFilter(f))
	default:
		panic(errors.ProgrammerError.New("unhandled filter mode %v", conf.MtimeMode))
	}
	return filterOptions
}

func EvaluateConfig(options ...MaterializerConfigurer) MaterializerOptions {
	var opts MaterializerOptions
	for _, each := range options {
		each(&opts)
	}
	return opts
}

type Placer func(srcPath, destPath string, writable bool, bareMount bool) Emplacement

type Emplacement interface {
	Teardown()
}

/*
	Assembles a filesystem from a bunch of scattered filesystem pieces.
	The source pieces will be rearranged into the single tree as fast as
	possible, leaving their original locations unchanged.

	`Assembler` is the boundary where you ask for what kind of relationship you want.
	Each `AssemblyPart` is a request to get someting into place; how exactly it
	gets there is up to the `Assembler` to decide.  (Fast systems may use
	bind mounts and COW filesystems to isolate and rearrange; worst-case
	scenario, plain ol' byte copies get the job done.)
*/
type Assembler func(basePath string, mounts []AssemblyPart) Assembly

type Assembly interface {
	Teardown()
}

type AssemblyPart struct {
	TargetPath string // in the container fs context
	SourcePath string // datasource which we want to respect
	/*
		Writable toggles whether or not to mount read-only.

		(If `Writable && BareMount`, then *this will allow modifications
		to the host*.)
	*/
	Writable bool
	/*
		BareMount requests direct passthrough; if writable==false, this
		means continuing changes in sourcepath are visible realtime; if
		writable==true, the placer will employ a bind mount *without*
		COW or isolation ** meaning mutations will be applied to the sourcepath **.
		Using a BareMount almost certainly means your job is giving up
		on any repeatability guarantees.

		Note that this may be impossible to satisfy with some placers
		(e.g. we can't give you a writable mount without the required
		sys caps to perform mounts!).
	*/
	BareMount bool
}

// sortable by target path (which is effectively mountability order)
type AssemblyPartsByPath []AssemblyPart

func (a AssemblyPartsByPath) Len() int           { return len(a) }
func (a AssemblyPartsByPath) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a AssemblyPartsByPath) Less(i, j int) bool { return a[i].TargetPath < a[j].TargetPath }
