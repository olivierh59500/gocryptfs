package frontend

import (
	"fmt"
	"github.com/rfjakob/gocryptfs/cryptfs"
	"github.com/rfjakob/cluefs/lib/cluefs"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

func fixFlags(flags fuse.OpenFlags) (fuse.OpenFlags, bool) {
	fmt.Printf("fixFlags: Before: %s\n", flags.String())
	var writeOnly bool
	// We always need read access to do read-modify-write cycles
	if flags & fuse.OpenWriteOnly > 0 {
		flags = flags &^ fuse.OpenWriteOnly
		flags = flags | fuse.OpenReadWrite
		writeOnly = true
	}
	// We also cannot open the file in append mode, we need to seek back for RMW
	flags = flags &^ fuse.OpenAppend
	fmt.Printf("fixFlags: After: %s\n", flags.String())
	return flags, writeOnly
}

func max(x int, y int) int {
	if x > y {
		return x
	}
	return y
}

type File struct {
	*cluefs.File
	crfs *cryptfs.CryptFS
	// Remember if the file is supposed to be write-only
	writeOnly bool
}

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fusefs.Handle, error) {
	fmt.Printf("File.Open\n")

	req.Flags, f.writeOnly = fixFlags(req.Flags)

	h, err := f.File.Open(ctx, req, resp)
	if err != nil {
		return nil, err
	}
	clueFile := h.(*cluefs.File)
	return &File {
		File: clueFile,
		crfs: f.crfs,
	}, nil
}

func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	iblocks := f.crfs.SplitRange(req.Offset, int64(req.Size))
	for _, ib := range iblocks {
		var partReq fuse.ReadRequest
		var partResp fuse.ReadResponse
		o, l := ib.CiphertextRange()
		partReq.Offset = o
		partReq.Size = int(l)
		partResp.Data = make([]byte, int(l))
		err := f.File.Read(ctx, &partReq, &partResp)
		if err != nil {
			return err
		}
		plaintext, err := f.crfs.DecryptBlock(partResp.Data)
		if err != nil {
			fmt.Printf("Read: Error reading block %d: %s\n", ib.BlockNo, err.Error())
			return err
		}
		plaintext = ib.CropBlock(plaintext)
		resp.Data = append(resp.Data, plaintext...)
	}
	return nil
}

func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	fmt.Printf("File.Write\n")
	resp.Size = 0
	iblocks := f.crfs.SplitRange(req.Offset, int64(len(req.Data)))
	var blockData []byte
	for _, ib := range iblocks {
		if ib.IsPartial() {
			// RMW
			blockData = make([]byte, f.crfs.PlainBS())
			var readReq fuse.ReadRequest
			var readResp fuse.ReadResponse
			o, l := ib.PlaintextRange()
			readReq.Offset = o
			readReq.Size = int(l)
			err := f.Read(ctx, &readReq, &readResp)
			if err != nil {
				return err
			}
			copy(blockData, readResp.Data)
			copy(blockData[ib.Offset:ib.Offset+ib.Length], req.Data)
			blockLen := max(len(readResp.Data), int(ib.Offset+ib.Length))
			blockData = blockData[0:blockLen]
		} else {
			blockData = req.Data[0:f.crfs.PlainBS()]
		}
		ciphertext := f.crfs.EncryptBlock(blockData)
		var partReq fuse.WriteRequest
		var partResp fuse.WriteResponse
		o, _ := ib.CiphertextRange()
		partReq.Data = ciphertext
		partReq.Offset = o
		err := f.File.Write(ctx, &partReq, &partResp)
		if err != nil {
			fmt.Printf("Write failure: %s\n", err.Error())
			return err
		}
		// Remove written data from the front of the request
		req.Data = req.Data[len(blockData):len(req.Data)]
		resp.Size += len(blockData)
	}
	return nil
}