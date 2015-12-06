package def

import (
	"sort"

	"github.com/ugorji/go/codec"
)

var _ codec.Selfer = &Env{}

func (e Env) CodecEncodeSelf(c *codec.Encoder) {
	c.Encode(e.asMappySlice())
}

func (mp Env) asMappySlice() codec.MapBySlice {
	keys := make([]string, len(mp))
	var i int
	for k := range mp {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	val := make(mappySlice, len(mp)*2)
	i = 0
	for _, k := range keys {
		val[i] = k
		i++
		val[i] = mp[k]
		i++
	}
	return val
}

func (e *Env) CodecDecodeSelf(c *codec.Decoder) {
	c.MustDecode((*map[string]string)(e))
}

var _ codec.Selfer = &MountGroup{}

func (mg MountGroup) CodecEncodeSelf(c *codec.Encoder) {
	// this one's a little unusual per the rest of the package, since
	//  for mountgroups we *are* using a slice as the canonical form.
	// also fun to note, mg is not currently required to already be
	//  be canonically sorted... because `AssembleFilesystem` does that
	//   after it's gathered all forms of mount info.
	sort.Sort(MountGroupByTargetPath(mg))
	var i int
	val := make(mappySlice, len(mg)*2)
	for _, k := range mg {
		val[i] = k.TargetPath
		i++
		val[i] = k.SourcePath
		i++
	}
	c.MustEncode(val)
}

func (mg *MountGroup) CodecDecodeSelf(c *codec.Decoder) {
	var raw map[string]interface{}
	c.MustDecode(&raw)
	var tmp MountGroup
	for k, v := range raw {
		m := Mount{
			TargetPath: k,
			SourcePath: v.(string),
			Writable:   true,
		}
		tmp = append(tmp, m)
	}
	(*mg) = tmp
}
