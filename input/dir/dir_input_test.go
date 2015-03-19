package dir

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"polydawn.net/repeatr/def"
	"polydawn.net/repeatr/input"
	"polydawn.net/repeatr/lib/fshash"
	"polydawn.net/repeatr/lib/fspatch"
	"polydawn.net/repeatr/testutil"
)

func Test(t *testing.T) {
	Convey("Given a nonexistant path", t, func() {
		Convey("The input config should be rejected during validation", func() {
			tryConstruction := func() {
				New(def.Input{
					Type: "dir",
					Hash: "abcd",
					URI:  "/tmp/certainly/should/not/exist",
				})
			}
			So(tryConstruction, testutil.ShouldPanicWith, def.ValidationError)
		})
	})

	testutil.Convey_IfHaveRoot("Given a directory with a mixture of files and folders", t,
		testutil.WithTmpdir(func() {
			pwd, _ := os.Getwd()
			os.Mkdir("src", 0755)
			os.Mkdir("src/a", 01777)
			os.Mkdir("src/b", 0750)
			f, err := os.OpenFile("src/b/c", os.O_RDWR|os.O_CREATE, 0664)
			So(err, ShouldBeNil)
			f.Write([]byte("zyx"))
			So(f.Close(), ShouldBeNil)

			// since we hash modtimes and this test has a fixture hash, we have to set those up!
			So(os.Chtimes("src", time.Unix(1, 2), time.Unix(1000, 2000)), ShouldBeNil)
			So(os.Chtimes("src/a", time.Unix(3, 2), time.Unix(3000, 2000)), ShouldBeNil)
			So(os.Chtimes("src/b", time.Unix(5, 2), time.Unix(5000, 2000)), ShouldBeNil)
			So(os.Chtimes("src/b/c", time.Unix(7, 2), time.Unix(7000, 2000)), ShouldBeNil)
			// similarly, force uid and gid bits since otherwise they default to your current user, and that's not the same for everyone
			So(os.Chown("src", 10000, 10000), ShouldBeNil)
			So(os.Chown("src/a", 10000, 10000), ShouldBeNil)
			So(os.Chown("src/b", 10000, 10000), ShouldBeNil)
			So(os.Chown("src/b/c", 10000, 10000), ShouldBeNil)

			fixtureHash := "nIf-ikfYp83OWWc_y2D-IGC9WOMYdfMA0l_11TL3VCeFq4QtsU6bBWeXyevujYr4"

			// save attributes first because access times are conceptually insane
			// remarkably, since the first read doesn't cause atimes to change,
			// the inputter can capture it and we can recreate it.
			// but that still doesn't make anything else about checking or handling it sane.
			path0metadata := fshash.ReadMetadata("src")
			path0metadata.Name = ""
			path1metadata := fshash.ReadMetadata("src/a")
			path2metadata := fshash.ReadMetadata("src/b")
			path3metadata := fshash.ReadMetadata("src/b/c")

			Convey("We can construct an input", func() {
				inputter := New(def.Input{
					Type: "dir",
					Hash: fixtureHash,
					URI:  filepath.Join(pwd, "src"),
				})

				Convey("Apply succeeds (hash fixture checks pass)", func() {
					waitCh := inputter.Apply(filepath.Join(pwd, "dest"))
					So(<-waitCh, ShouldBeNil)

					Convey("The destination files exist", func() {
						So("dest/a", testutil.ShouldBeFile, os.ModeDir)
						So("dest/b", testutil.ShouldBeFile, os.ModeDir)
						So("dest/b/c", testutil.ShouldBeFile, os.FileMode(0))
						content, err := ioutil.ReadFile("dest/b/c")
						So(err, ShouldBeNil)
						So(string(content), ShouldEqual, "zyx")

						Convey("And all metadata matches", func() {
							// Comparing fileinfo doesn't work conveniently; you keep getting new pointers for 'sys'
							//one, _ := os.Lstat("src/a")
							//two, _ := os.Lstat("dest/a")
							//So(one, ShouldResemble, two)
							So(fshash.ReadMetadata("dest/a"), ShouldResemble, path1metadata)
							So(fshash.ReadMetadata("dest/b"), ShouldResemble, path2metadata)
							So(fshash.ReadMetadata("dest/b/c"), ShouldResemble, path3metadata)
							// the top dir should have the same attribs too!  but we have to fix the name.
							destDirMeta := fshash.ReadMetadata("dest/")
							destDirMeta.Name = ""
							So(destDirMeta, ShouldResemble, path0metadata)
						})
					})

					Convey("Copying the copy should still match on hash", func() {
						inputter2 := New(def.Input{
							Type: "dir",
							Hash: fixtureHash,
							URI:  filepath.Join(pwd, "dest"),
						})

						waitCh := inputter2.Apply(filepath.Join(pwd, "copycopy"))
						So(<-waitCh, ShouldBeNil)
					})
				})
			})

			Convey("A different hash is rejected", func() {
				inputter := New(def.Input{
					Type: "dir",
					Hash: "abcd",
					URI:  filepath.Join(pwd, "src"),
				})
				err := <-inputter.Apply(filepath.Join(pwd, "dest"))
				So(err, testutil.ShouldBeErrorClass, input.InputHashMismatchError)
			})

			Convey("A change in content breaks the hash", func() {
				// we could do separate tests for added and removed, but those don't trigger markedly different paths so i think we're pretty well covered already.
				inputter := New(def.Input{
					Type: "dir",
					Hash: fixtureHash,
					URI:  filepath.Join(pwd, "src"),
				})
				f, err := os.OpenFile("src/b/c", os.O_RDWR, 0664)
				So(err, ShouldBeNil)
				f.Write([]byte("222"))
				So(f.Close(), ShouldBeNil)
				err = <-inputter.Apply(filepath.Join(pwd, "dest"))
				So(err, testutil.ShouldBeErrorClass, input.InputHashMismatchError)
			})
		}),
	)

	// be advised this is mostly a copypasta of above test with a different filesystem -- we should really make them parameterized tests!
	// this set also raises the bar on UIDs and GIDs because we want to make sure we test setting those on symlinks doesn't go through.
	testutil.Convey_IfHaveRoot("Given a directory with a mixture of files, folders, and symlinks", t,
		testutil.WithTmpdir(func() {
			pwd, _ := os.Getwd()
			os.Mkdir("src", 0755)
			os.Mkdir("src/a", 01777)
			os.Mkdir("src/b", 0750)
			f, err := os.OpenFile("src/b/c", os.O_RDWR|os.O_CREATE, 0664)
			So(err, ShouldBeNil)
			f.Write([]byte("zyx"))
			So(f.Close(), ShouldBeNil)
			os.Mkdir("src/b/d", 0755)
			os.Symlink("../c", "src/b/d/link-rel")
			os.Symlink("/tmp/nonexistant/have-mercy", "src/link-abs")

			// since we hash modtimes and this test has a fixture hash, we have to set those up!
			So(os.Chtimes("src", time.Unix(1, 2), time.Unix(1000, 2000)), ShouldBeNil)
			So(os.Chtimes("src/a", time.Unix(3, 2), time.Unix(3000, 2000)), ShouldBeNil)
			So(os.Chtimes("src/b", time.Unix(5, 2), time.Unix(5000, 2000)), ShouldBeNil)
			So(os.Chtimes("src/b/c", time.Unix(7, 2), time.Unix(7000, 2000)), ShouldBeNil)
			So(os.Chtimes("src/b/d", time.Unix(9, 2), time.Unix(9000, 2000)), ShouldBeNil)
			So(fspatch.LUtimesNano("src/b/d/link-rel", []syscall.Timespec{syscall.NsecToTimespec(time.Unix(11, 2).UnixNano()), syscall.NsecToTimespec(time.Unix(11000, 2000).UnixNano())}), ShouldBeNil)
			So(fspatch.LUtimesNano("src/link-abs", []syscall.Timespec{syscall.NsecToTimespec(time.Unix(11, 2).UnixNano()), syscall.NsecToTimespec(time.Unix(11000, 2000).UnixNano())}), ShouldBeNil)
			// similarly, force uid and gid bits since otherwise they default to your current user, and that's not the same for everyone
			So(os.Chown("src", 10000, 10000), ShouldBeNil)
			So(os.Chown("src/a", 10001, 10001), ShouldBeNil)
			So(os.Chown("src/b", 10000, 10000), ShouldBeNil)
			So(os.Chown("src/b/c", 10000, 10000), ShouldBeNil)
			So(os.Chown("src/b/d", 10000, 10000), ShouldBeNil)
			So(os.Lchown("src/b/d/link-rel", 10002, 10002), ShouldBeNil)
			So(os.Lchown("src/link-abs", 10000, 10000), ShouldBeNil)

			fixtureHash := "SQ013PmxYZ6ofOZ_sFm4fx_bQDmJAjSMn88OZ7gm_Z-Vo_iGhlEt-fVYafp1aJXz"

			Convey("We can construct an input", func() {
				inputter := New(def.Input{
					Type: "dir",
					Hash: fixtureHash,
					URI:  filepath.Join(pwd, "src"),
				})

				Convey("Apply succeeds (hash fixture checks pass)", func() {
					waitCh := inputter.Apply(filepath.Join(pwd, "dest"))
					So(<-waitCh, ShouldBeNil)

					Convey("The destination files exist", func() {
						So("dest/a", testutil.ShouldBeFile, os.ModeDir)
						So("dest/b", testutil.ShouldBeFile, os.ModeDir)
						So("dest/b/c", testutil.ShouldBeFile, os.FileMode(0))
						content, err := ioutil.ReadFile("dest/b/c")
						So(err, ShouldBeNil)
						So(string(content), ShouldEqual, "zyx")

						Convey("And all metadata matches", func() {
							So(fshash.ReadMetadata("dest/a"), ShouldResemble, fshash.ReadMetadata("src/a"))
							So(fshash.ReadMetadata("dest/b"), ShouldResemble, fshash.ReadMetadata("src/b"))
							So(fshash.ReadMetadata("dest/b/c"), ShouldResemble, fshash.ReadMetadata("src/b/c"))
							So(fshash.ReadMetadata("dest/b/d"), ShouldResemble, fshash.ReadMetadata("src/b/d"))
							So(fshash.ReadMetadata("dest/b/d/link-rel"), ShouldResemble, fshash.ReadMetadata("src/b/d/link-rel"))
							So(fshash.ReadMetadata("dest/link-abs"), ShouldResemble, fshash.ReadMetadata("src/link-abs"))
							// the top dir should have the same attribs too!  but we have to fix the name.
							srcDirMetadata := fshash.ReadMetadata("src/")
							srcDirMetadata.Name = ""
							destDirMeta := fshash.ReadMetadata("dest/")
							destDirMeta.Name = ""
							So(destDirMeta, ShouldResemble, srcDirMetadata)
						})

						Convey("The symlink should be readable", func() {
							// just covering the relative one
							content, err := ioutil.ReadFile("dest/b/d/link-rel")
							So(err, ShouldBeNil)
							So(string(content), ShouldEqual, "zyx")
						})
					})

					Convey("Copying the copy should still match on hash", func() {
						inputter2 := New(def.Input{
							Type: "dir",
							Hash: fixtureHash,
							URI:  filepath.Join(pwd, "dest"),
						})

						waitCh := inputter2.Apply(filepath.Join(pwd, "copycopy"))
						So(<-waitCh, ShouldBeNil)
					})
				})
			})

			Convey("A different hash is rejected", func() {
				inputter := New(def.Input{
					Type: "dir",
					Hash: "abcd",
					URI:  filepath.Join(pwd, "src"),
				})
				err := <-inputter.Apply(filepath.Join(pwd, "dest"))
				So(err, testutil.ShouldBeErrorClass, input.InputHashMismatchError)
			})

			Convey("A change in content breaks the hash", func() {
				// we could do separate tests for added and removed, but those don't trigger markedly different paths so i think we're pretty well covered already.
				inputter := New(def.Input{
					Type: "dir",
					Hash: fixtureHash,
					URI:  filepath.Join(pwd, "src"),
				})
				f, err := os.OpenFile("src/b/c", os.O_RDWR, 0664)
				So(err, ShouldBeNil)
				f.Write([]byte("222"))
				So(f.Close(), ShouldBeNil)
				err = <-inputter.Apply(filepath.Join(pwd, "dest"))
				So(err, testutil.ShouldBeErrorClass, input.InputHashMismatchError)
			})
		}),
	)
}
