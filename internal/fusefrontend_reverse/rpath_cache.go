package fusefrontend_reverse

import (
	"sync"
)

// rPathCacheContainer is a simple one entry path cache. Because the dirIV
// is generated deterministically from the directory path, there is no need
// to ever invalidate entries.
type rPathCacheContainer struct {
	sync.Mutex
	// Relative ciphertext path to the directory
	cPath string
	// Relative plaintext path
	pPath string
	// Directory IV of the directory
	dirIV []byte
}

func (c *rPathCacheContainer) lookup(cPath string) ([]byte, string) {
	c.Lock()
	defer c.Unlock()
	if cPath == c.cPath {
		//fmt.Printf("HIT   %q\n", cPath)
		return c.dirIV, c.pPath
	}
	//fmt.Printf("MISS  %q\n", cPath)
	return nil, ""
}

// store - write entry for "cPath" into the cache
func (c *rPathCacheContainer) store(cPath string, dirIV []byte, pPath string) {
	//fmt.Printf("STORE %q\n", cPath)
	c.Lock()
	defer c.Unlock()
	c.cPath = cPath
	c.dirIV = dirIV
	c.pPath = pPath
}

var rPathCache rPathCacheContainer
