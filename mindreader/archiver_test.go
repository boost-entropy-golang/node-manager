// Copyright 2019 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mindreader

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/merger/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiver_StoreBlockNewBlocks(t *testing.T) {
	io := &TestArchiverIO{}
	superLongTimeAgo := time.Since(time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC))
	archiver := NewArchiver(5, io, false, nil, superLongTimeAgo, testLogger)

	srcOneBlockFiles := []*bundle.OneBlockFile{
		bundle.MustNewOneBlockFile("0000000001-20210728T105016.01-00000001a-00000000a-0-suffix"),
		bundle.MustNewOneBlockFile("0000000002-20210728T105016.02-00000002a-00000001a-0-suffix"),
		bundle.MustNewOneBlockFile("0000000003-20210728T105016.03-00000003a-00000002a-0-suffix"),
		bundle.MustNewOneBlockFile("0000000004-20210728T105016.06-00000004a-00000003a-2-suffix"),
		bundle.MustNewOneBlockFile("0000000006-20210728T105016.08-00000006a-00000004a-2-suffix"),
	}

	storedMergableOneBlockFiles := 0
	io.StoreMergeableOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedMergableOneBlockFiles++
		return nil
	}

	storedUploadableOneBlockfiles := 0
	io.StoreOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedUploadableOneBlockfiles++
		return nil
	}

	storedMergedFiles := 0
	io.MergeAndStoreFunc = func(inclusiveLowerBlock uint64, oneBlockFiles []*bundle.OneBlockFile) (err error) {
		storedMergedFiles++
		return nil
	}

	deletedFiles := 0
	io.DeleteOneBlockFilesFunc = func(oneBlockFiles []*bundle.OneBlockFile) {
		deletedFiles += len(oneBlockFiles)
	}

	ctx := context.Background()
	for _, oneBlockFile := range srcOneBlockFiles {
		err := archiver.storeBlock(ctx, oneBlockFile, oneBlockFileToBlock(oneBlockFile))
		require.NoError(t, err)
	}

	assert.Equal(t, 0, storedMergedFiles)
	assert.Equal(t, 0, deletedFiles)
	assert.Equal(t, 0, storedMergableOneBlockFiles)
	assert.Equal(t, 5, storedUploadableOneBlockfiles)
}

func TestArchiver_StoreBlockNewBlocksWithExistingBundlerBlocks(t *testing.T) {
	setter := bstream.GetBlockPayloadSetter
	bstream.GetBlockPayloadSetter = bstream.MemoryBlockPayloadSetter
	defer func() {
		bstream.GetBlockPayloadSetter = setter
	}()

	io := &TestArchiverIO{}
	superLongTimeAgo := time.Since(time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC))
	archiver := NewArchiver(5, io, false, nil, superLongTimeAgo, testLogger)

	bundlerOneBlockFiles := []*bundle.OneBlockFile{
		bundle.MustNewOneBlockFile("0000000001-20210728T105016.01-00000001a-00000000a-0-suffix"),
		bundle.MustNewOneBlockFile("0000000002-20210728T105016.02-00000002a-00000001a-0-suffix"),
	}

	bundler := bundle.NewBundler(5, math.MaxUint64)
	for _, obf := range bundlerOneBlockFiles {
		bundler.AddOneBlockFile(obf)
	}
	archiver.bundler = bundler

	srcOneBlockFiles := []*bundle.OneBlockFile{
		bundle.MustNewOneBlockFile("0000000003-20210728T105016.03-00000003a-00000002a-0-suffix"),
		bundle.MustNewOneBlockFile("0000000004-20210728T105016.06-00000004a-00000003a-2-suffix"),
		bundle.MustNewOneBlockFile("0000000006-20210728T105016.08-00000006a-00000004a-2-suffix"),
	}

	io.DownloadOneBlockFileFunc = func(ctx context.Context, oneBlockFile *bundle.OneBlockFile) (data []byte, err error) {
		blk := oneBlockFileToBlock(oneBlockFile)
		blk, err = bstream.MemoryBlockPayloadSetter(blk, []byte(oneBlockFile.CanonicalName))
		if err != nil {
			return nil, err
		}

		pblk, err := blk.ToProto()
		if err != nil {
			return nil, err
		}
		return proto.Marshal(pblk)
	}

	storedMergableOneBlockFiles := 0
	io.StoreMergeableOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedMergableOneBlockFiles++
		return nil
	}

	storedUploadableOneBlockfiles := 0
	io.StoreOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedUploadableOneBlockfiles++
		return nil
	}

	storedMergedFiles := 0
	io.MergeAndStoreFunc = func(inclusiveLowerBlock uint64, oneBlockFiles []*bundle.OneBlockFile) (err error) {
		storedMergedFiles++
		return nil
	}

	deletedFiles := 0
	io.DeleteOneBlockFilesFunc = func(oneBlockFiles []*bundle.OneBlockFile) {
		deletedFiles += len(oneBlockFiles)
	}

	ctx := context.Background()
	for _, oneBlockFile := range srcOneBlockFiles {
		err := archiver.storeBlock(ctx, oneBlockFile, oneBlockFileToBlock(oneBlockFile))
		require.NoError(t, err)
	}

	assert.Equal(t, 0, storedMergedFiles)
	assert.Equal(t, 0, deletedFiles)
	assert.Equal(t, 0, storedMergableOneBlockFiles)
	assert.Equal(t, 5, storedUploadableOneBlockfiles)
}

func TestArchiver_StoreBlock_OldBlocksPassThroughBoundary(t *testing.T) {
	io := &TestArchiverIO{}
	archiver := NewArchiver(5, io, false, nil, time.Hour, testLogger)

	srcOneBlockFiles := []*bundle.OneBlockFile{
		bundle.MustNewOneBlockFile("0000000001-20210728T105016.01-00000001a-00000000a-0-suffix"),
		bundle.MustNewOneBlockFile("0000000002-20210728T105016.02-00000002a-00000001a-0-suffix"),
		bundle.MustNewOneBlockFile("0000000003-20210728T105016.03-00000003a-00000002a-0-suffix"),
		bundle.MustNewOneBlockFile("0000000004-20210728T105016.06-00000004a-00000003a-2-suffix"),
		bundle.MustNewOneBlockFile("0000000006-20210728T105016.08-00000006a-00000004a-2-suffix"),
	}

	storedMergableOneBlockFiles := 0
	io.StoreMergeableOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedMergableOneBlockFiles++
		return nil
	}

	storedUploadableOneBlockfiles := 0
	io.StoreOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedUploadableOneBlockfiles++
		return nil
	}

	storedMergedFiles := 0
	io.MergeAndStoreFunc = func(inclusiveLowerBlock uint64, oneBlockFiles []*bundle.OneBlockFile) (err error) {
		storedMergedFiles++
		return nil
	}

	deletedFiles := 0
	io.DeleteOneBlockFilesFunc = func(oneBlockFiles []*bundle.OneBlockFile) {
		deletedFiles += len(oneBlockFiles)
	}

	ctx := context.Background()
	for _, oneBlockFile := range srcOneBlockFiles {
		err := archiver.storeBlock(ctx, oneBlockFile, oneBlockFileToBlock(oneBlockFile))
		require.NoError(t, err)
	}

	assert.Equal(t, 1, storedMergedFiles)
	assert.Equal(t, 4, deletedFiles)
	assert.Equal(t, 5, storedMergableOneBlockFiles)
	assert.Equal(t, 0, storedUploadableOneBlockfiles)
}

func TestArchiver_StoreBlock_BundleInclusiveLowerBlock(t *testing.T) {
	io := &TestArchiverIO{}
	archiver := NewArchiver(5, io, false, nil, time.Hour, testLogger)

	srcOneBlockFiles := []*bundle.OneBlockFile{
		bundle.MustNewOneBlockFile("00000000011-20210728T105016.01-000000011a-000000010a-10-suffix"),

		bundle.MustNewOneBlockFile("00000000005-20210728T105015.00-0000000005a-000000004a-01-suffix"),

		bundle.MustNewOneBlockFile("00000000012-20210728T105016.02-000000012a-000000011a-10-suffix"),
		bundle.MustNewOneBlockFile("00000000013-20210728T105016.03-000000013a-000000012a-10-suffix"),
		bundle.MustNewOneBlockFile("00000000014-20210728T105016.06-000000014a-000000013a-12-suffix"),
		bundle.MustNewOneBlockFile("00000000016-20210728T105016.08-000000016a-000000014a-12-suffix"),
	}

	storedMergableOneBlockFiles := 0
	io.StoreMergeableOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedMergableOneBlockFiles++
		return nil
	}

	storedUploadableOneBlockfiles := 0
	io.StoreOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedUploadableOneBlockfiles++
		return nil
	}

	storedMergedFiles := 0
	io.MergeAndStoreFunc = func(inclusiveLowerBlock uint64, oneBlockFiles []*bundle.OneBlockFile) (err error) {
		storedMergedFiles++
		return nil
	}

	deletedFiles := 0
	io.DeleteOneBlockFilesFunc = func(oneBlockFiles []*bundle.OneBlockFile) {
		deletedFiles += len(oneBlockFiles)
	}

	ctx := context.Background()
	for _, oneBlockFile := range srcOneBlockFiles {
		err := archiver.storeBlock(ctx, oneBlockFile, oneBlockFileToBlock(oneBlockFile))
		require.NoError(t, err)
	}

	assert.Equal(t, 1, storedMergedFiles)
	assert.Equal(t, 4, deletedFiles)
	assert.Equal(t, 5, storedMergableOneBlockFiles)
	assert.Equal(t, 0, storedUploadableOneBlockfiles)
}

func TestArchiver_Store_OneBlock_after_last_merge(t *testing.T) {
	bstream.GetBlockPayloadSetter = bstream.MemoryBlockPayloadSetter
	io := &TestArchiverIO{}
	archiver := NewArchiver(5, io, false, nil, time.Hour, testLogger)

	srcOneBlockFiles := []*bundle.OneBlockFile{
		bundle.MustNewOneBlockFile("00000000011-20210728T105016.01-000000011a-000000010a-10-suffix"),
		bundle.MustNewOneBlockFile("00000000012-20210728T105016.02-000000012a-000000011a-10-suffix"),
		bundle.MustNewOneBlockFile("00000000013-20210728T105016.03-000000013a-000000012a-10-suffix"),
		bundle.MustNewOneBlockFile("00000000014-20210728T105016.06-000000014a-000000013a-12-suffix"),
		bundle.MustNewOneBlockFile("00000000016-20210728T105016.08-000000016a-000000014a-12-suffix"),
		bundle.MustNewOneBlockFile("00000000017-20210728T105016.08-000000017a-000000016a-12-suffix"),
	}
	io.DownloadOneBlockFileFunc = func(ctx context.Context, oneBlockFile *bundle.OneBlockFile) (data []byte, err error) {
		block := oneBlockFileToBlock(oneBlockFile)
		block, err = bstream.MemoryBlockPayloadSetter(block, []byte(oneBlockFile.CanonicalName))
		if err != nil {
			return nil, err
		}

		pbBlock, err := block.ToProto()
		if err != nil {
			return nil, err
		}
		return proto.Marshal(pbBlock)

	}

	storedMergableOneBlockFiles := 0
	io.StoreMergeableOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedMergableOneBlockFiles++
		return nil
	}

	storedUploadableOneBlockFiles := 0
	io.StoreOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedUploadableOneBlockFiles++
		return nil
	}

	storedMergedFiles := 0
	io.MergeAndStoreFunc = func(inclusiveLowerBlock uint64, oneBlockFiles []*bundle.OneBlockFile) (err error) {
		storedMergedFiles++
		return nil
	}

	deletedFiles := 0
	io.DeleteOneBlockFilesFunc = func(oneBlockFiles []*bundle.OneBlockFile) {
		deletedFiles += len(oneBlockFiles)
	}

	ctx := context.Background()
	for i, oneBlockFile := range srcOneBlockFiles {
		err := archiver.storeBlock(ctx, oneBlockFile, oneBlockFileToBlock(oneBlockFile))
		if i == 4 {
			archiver.currentlyMerging = false //force the end off merging state.
		}
		require.NoError(t, err)
	}

	assert.Equal(t, 1, storedMergedFiles)
	assert.Equal(t, 4, deletedFiles)
	assert.Equal(t, 5, storedMergableOneBlockFiles)
	assert.Equal(t, 2, storedUploadableOneBlockFiles)
}

func TestArchiver_StoreBlock_NewBlocksBatchMode(t *testing.T) {
	io := &TestArchiverIO{}
	archiver := NewArchiver(5, io, true, nil, time.Hour, testLogger)

	srcExistingMergeableOneBlockFiles := []string{
		"0000000001-20210728T105016.01-00000001a-00000000a-0-suffix",
		"0000000002-20210728T105016.02-00000002a-00000001a-1-suffix",
	}

	io.WalkMergeableOneBlockFilesFunc = func(ctx context.Context) ([]*bundle.OneBlockFile, error) {
		result := []*bundle.OneBlockFile{}
		for _, filename := range srcExistingMergeableOneBlockFiles {
			obf := bundle.MustNewOneBlockFile(filename)
			_, _, _, _, libNumPtr, _, _ := bundle.ParseFilename(filename)
			obf.InnerLibNum = libNumPtr
			result = append(result, obf)
		}
		return result, nil
	}

	srcOneBlockFiles := []*bundle.OneBlockFile{
		bundle.MustNewOneBlockFile("0000000003-20210728T105016.03-00000003a-00000002a-1-suffix"),
		bundle.MustNewOneBlockFile("0000000004-20210728T105016.06-00000004a-00000003a-2-suffix"),
		bundle.MustNewOneBlockFile("0000000006-20210728T105016.08-00000006a-00000004a-2-suffix"),
	}

	storedMergableOneBlockFiles := 0
	io.StoreMergeableOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedMergableOneBlockFiles++
		return nil
	}

	storedUploadableOneBlockFiles := 0
	io.StoreOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedUploadableOneBlockFiles++
		return nil
	}

	storedMergedFiles := 0
	io.MergeAndStoreFunc = func(inclusiveLowerBlock uint64, oneBlockFiles []*bundle.OneBlockFile) (err error) {
		storedMergedFiles++
		return nil
	}

	deletedFiles := 0
	io.DeleteOneBlockFilesFunc = func(oneBlockFiles []*bundle.OneBlockFile) {
		deletedFiles += len(oneBlockFiles)
	}

	ctx := context.Background()
	for _, oneBlockFile := range srcOneBlockFiles {
		err := archiver.storeBlock(ctx, oneBlockFile, oneBlockFileToBlock(oneBlockFile))
		require.NoError(t, err)
	}

	assert.Equal(t, 1, storedMergedFiles)
	assert.Equal(t, 4, deletedFiles)
	assert.Equal(t, 3, storedMergableOneBlockFiles)
	assert.Equal(t, 0, storedUploadableOneBlockFiles)
}

func TestArchiver_StoreBlock_NewBlocksBatchModeNonConnectedPartial_MultipleBoundaries(t *testing.T) {
	io := &TestArchiverIO{}
	archiver := NewArchiver(5, io, true, nil, time.Hour, testLogger)

	srcExistingMergeableOneBlockFiles := []string{
		"0000000001-20210728T105016.01-00000001a-00000000a-0-suffix",
		"0000000002-20210728T105016.02-00000002a-00000001a-1-suffix",
	}

	io.WalkMergeableOneBlockFilesFunc = func(ctx context.Context) ([]*bundle.OneBlockFile, error) {
		result := []*bundle.OneBlockFile{}
		for _, filename := range srcExistingMergeableOneBlockFiles {
			obf := bundle.MustNewOneBlockFile(filename)
			_, _, _, _, libNumPtr, _, _ := bundle.ParseFilename(filename)
			obf.InnerLibNum = libNumPtr
			result = append(result, obf)
		}
		return result, nil
	}

	srcOneBlockFiles := []*bundle.OneBlockFile{
		//bundle.MustNewOneBlockFile("0000000003-20210728T105016.03-00000003a-00000002a-1-suffix"),
		bundle.MustNewOneBlockFile("0000000004-20210728T105016.06-00000004a-00000003a-1-suffix"),
		bundle.MustNewOneBlockFile("0000000006-20210728T105016.08-00000006a-00000004a-4-suffix"),
		bundle.MustNewOneBlockFile("0000000007-20210728T105016.09-00000007a-00000006a-4-suffix"),
		bundle.MustNewOneBlockFile("0000000009-20210728T105016.09-00000009a-00000007a-6-suffix"),
		bundle.MustNewOneBlockFile("0000000010-20210728T105016.09-00000010a-00000009a-6-suffix"),
		bundle.MustNewOneBlockFile("0000000011-20210728T105016.09-00000011a-00000010a-9-suffix"),
	}

	storedMergableOneBlockFiles := 0
	io.StoreMergeableOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedMergableOneBlockFiles++
		return nil
	}

	storedUploadableOneBlockfiles := 0
	io.StoreOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedUploadableOneBlockfiles++
		return nil
	}

	storedMergedFiles := 0
	io.MergeAndStoreFunc = func(inclusiveLowerBlock uint64, oneBlockFiles []*bundle.OneBlockFile) (err error) {
		storedMergedFiles++
		return nil
	}

	deletedFiles := 0
	io.DeleteOneBlockFilesFunc = func(oneBlockFiles []*bundle.OneBlockFile) {
		deletedFiles += len(oneBlockFiles)
	}

	ctx := context.Background()
	for _, oneBlockFile := range srcOneBlockFiles {
		err := archiver.storeBlock(ctx, oneBlockFile, oneBlockFileToBlock(oneBlockFile))
		require.NoError(t, err)
	}

	assert.Equal(t, 2, storedMergedFiles)
	assert.Equal(t, 4, deletedFiles)
	assert.Equal(t, 6, storedMergableOneBlockFiles)
	assert.Equal(t, 0, storedUploadableOneBlockfiles)
}

func TestArchiver_OldBlockToNewBlocksPassThrough(t *testing.T) {
	setter := bstream.GetBlockPayloadSetter
	bstream.GetBlockPayloadSetter = bstream.MemoryBlockPayloadSetter
	defer func() {
		bstream.GetBlockPayloadSetter = setter
	}()

	io := &TestArchiverIO{}
	archiver := NewArchiver(5, io, false, nil, 24*time.Hour, testLogger)

	time.Now().Year()
	yearstr := fmt.Sprintf("%0*d", 4, time.Now().Year())
	monthstr := fmt.Sprintf("%0*d", 2, time.Now().Month())
	daystr := fmt.Sprintf("%0*d", 2, time.Now().Day())
	hourstr := fmt.Sprintf("%0*d", 2, time.Now().Hour())
	minutestr := fmt.Sprintf("%0*d", 2, time.Now().Minute())
	secondstr := fmt.Sprintf("%0*d", 2, time.Now().Second())
	nowstr := fmt.Sprintf("%s%s%sT%s%s%s", yearstr, monthstr, daystr, hourstr, minutestr, secondstr)

	srcOneBlockFiles := []*bundle.OneBlockFile{
		bundle.MustNewOneBlockFile("0000000001-20000728T105016.01-00000001a-00000000a-0-suffix"), //old block
		bundle.MustNewOneBlockFile(fmt.Sprintf("0000000002-%s.02-00000002a-00000001a-1-suffix", nowstr)),
		bundle.MustNewOneBlockFile(fmt.Sprintf("0000000003-%s.03-00000003a-00000002a-1-suffix", nowstr)),
		bundle.MustNewOneBlockFile(fmt.Sprintf("0000000004-%s.06-00000004a-00000003a-2-suffix", nowstr)),
		bundle.MustNewOneBlockFile(fmt.Sprintf("0000000006-%s.08-00000006a-00000004a-2-suffix", nowstr)),
		bundle.MustNewOneBlockFile(fmt.Sprintf("0000000006-%s.09-00000007a-00000006a-2-suffix", nowstr)),
		bundle.MustNewOneBlockFile(fmt.Sprintf("0000000006-%s.10-00000008a-00000007a-2-suffix", nowstr)),
		bundle.MustNewOneBlockFile(fmt.Sprintf("0000000006-%s.11-00000009a-00000008a-2-suffix", nowstr)),
	}

	io.DownloadOneBlockFileFunc = func(ctx context.Context, oneBlockFile *bundle.OneBlockFile) (data []byte, err error) {
		blk := oneBlockFileToBlock(oneBlockFile)
		blk, err = bstream.MemoryBlockPayloadSetter(blk, []byte(oneBlockFile.CanonicalName))
		if err != nil {
			return nil, err
		}

		pblk, err := blk.ToProto()
		if err != nil {
			return nil, err
		}
		return proto.Marshal(pblk)
	}

	storedMergableOneBlockFiles := 0
	io.StoreMergeableOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedMergableOneBlockFiles++
		return nil
	}

	storedUploadableOneBlockfiles := 0
	io.StoreOneBlockFileFunc = func(ctx context.Context, fileName string, block *bstream.Block) error {
		storedUploadableOneBlockfiles++
		return nil
	}

	storedMergedFiles := 0
	io.MergeAndStoreFunc = func(inclusiveLowerBlock uint64, oneBlockFiles []*bundle.OneBlockFile) (err error) {
		storedMergedFiles++
		return nil
	}

	deletedFiles := 0
	io.DeleteOneBlockFilesFunc = func(oneBlockFiles []*bundle.OneBlockFile) {
		deletedFiles += len(oneBlockFiles)
	}

	ctx := context.Background()
	for _, oneBlockFile := range srcOneBlockFiles {
		err := archiver.storeBlock(ctx, oneBlockFile, oneBlockFileToBlock(oneBlockFile))
		require.NoError(t, err)
	}

	assert.Equal(t, 0, storedMergedFiles)
	assert.Equal(t, 0, deletedFiles)
	assert.Equal(t, 1, storedMergableOneBlockFiles)
	assert.Equal(t, 8, storedUploadableOneBlockfiles)
}

func oneBlockFileToBlock(oneBlockFile *bundle.OneBlockFile) *bstream.Block {
	return &bstream.Block{
		Id:             oneBlockFile.ID,
		Number:         oneBlockFile.Num,
		PreviousId:     oneBlockFile.PreviousID,
		Timestamp:      oneBlockFile.BlockTime,
		LibNum:         oneBlockFile.LibNum(),
		PayloadKind:    0,
		PayloadVersion: 0,
		Payload:        nil,
	}
}
