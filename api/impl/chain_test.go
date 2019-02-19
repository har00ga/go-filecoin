package impl

import (
	"context"
	"encoding/json"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"testing"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChainHead(t *testing.T) {
	t.Parallel()
	t.Run("returns an error if no best block", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		n := node.MakeOfflineNode(t)
		api := New(n)

		_, err := api.Chain().Head()

		require.Error(err)
		require.EqualError(err, ErrHeaviestTipSetNotFound.Error())
	})

	t.Run("emits the blockchain head", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		require := require.New(t)
		assert := assert.New(t)

		blk := types.NewBlockForTest(nil, 1)
		n := node.MakeOfflineNode(t)
		chainStore, ok := n.ChainReader.(chain.Store)
		require.True(ok)

		chainStore.SetHead(ctx, testhelpers.RequireNewTipSet(require, blk))

		api := New(n)
		out, err := api.Chain().Head()

		require.NoError(err)
		assert.Len(out, 1)
		types.AssertCidsEqual(assert, out[0], blk.Cid())
	})

	t.Run("the blockchain head is sorted", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		require := require.New(t)
		assert := assert.New(t)

		blk := types.NewBlockForTest(nil, 0)
		blk2 := types.NewBlockForTest(nil, 1)
		blk3 := types.NewBlockForTest(nil, 2)

		n := node.MakeOfflineNode(t)
		chainStore, ok := n.ChainReader.(chain.Store)
		require.True(ok)

		newTipSet := testhelpers.RequireNewTipSet(require, blk)
		testhelpers.RequireTipSetAdd(require, blk2, newTipSet)
		testhelpers.RequireTipSetAdd(require, blk3, newTipSet)

		someErr := chainStore.SetHead(ctx, newTipSet)
		require.NoError(someErr)

		api := New(n)
		out, err := api.Chain().Head()
		require.NoError(err)

		sortedCidSet := types.NewSortedCidSet(blk.Cid(), blk2.Cid(), blk3.Cid())

		assert.Len(out, 3)
		assert.Equal(sortedCidSet.ToSlice(), out)
	})
}

func TestChainLsRun(t *testing.T) {
	t.Parallel()
	t.Run("chain of height two", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)
		assert := assert.New(t)

		ctx := context.Background()
		n := node.MakeOfflineNode(t)

		chainStore, ok := n.ChainReader.(chain.Store)
		require.True(ok)
		genBlock, err := consensus.InitGenesis(n.CborStore(), n.Blockstore)
		require.NoError(err)

		chlBlock := types.NewBlockForTest(genBlock, 1)
		chlTS := testhelpers.RequireNewTipSet(require, chlBlock)
		err = chainStore.PutTipSetAndState(ctx, &chain.TipSetAndState{
			TipSet:          chlTS,
			TipSetStateRoot: chlBlock.StateRoot,
		})
		require.NoError(err)
		err = chainStore.SetHead(ctx, chlTS)
		require.NoError(err)

		api := New(n)

		var bs [][]*types.Block
		for raw := range api.Chain().Ls(ctx) {
			switch v := raw.(type) {
			case consensus.TipSet:
				bs = append(bs, v.ToSlice())
			default:
				assert.FailNow("invalid element in ls", v)
			}
		}

		assert.Equal(2, len(bs))
		types.AssertHaveSameCid(assert, chlBlock, bs[0][0])
		types.AssertHaveSameCid(assert, genBlock, bs[1][0])
	})

	t.Run("emit best block and then time out getting parent", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		ctx := context.Background()
		n := node.MakeOfflineNode(t)

		parBlock := types.NewBlockForTest(nil, 0)
		chlBlock := types.NewBlockForTest(parBlock, 1)

		chainStore, ok := n.ChainReader.(chain.Store)
		require.True(ok)
		chlTS := testhelpers.RequireNewTipSet(require, chlBlock)
		err := chainStore.PutTipSetAndState(ctx, &chain.TipSetAndState{
			TipSet:          chlTS,
			TipSetStateRoot: chlBlock.StateRoot,
		})
		require.NoError(err)
		err = chainStore.SetHead(ctx, chlTS)
		require.NoError(err)

		api := New(n)
		// parBlock is not known to the chain, which causes the timeout
		var innerErr error
		for raw := range api.Chain().Ls(ctx) {
			switch v := raw.(type) {
			case error:
				innerErr = v
			case consensus.TipSet:
				// ignore
			default:
				require.FailNow("invalid element in ls", v)
			}
		}

		require.NotNil(innerErr)
		require.Contains(innerErr.Error(), "failed to get block")
	})

	t.Run("JSON marshaling", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		parent := types.NewBlockForTest(nil, 0)
		child := types.NewBlockForTest(parent, 1)

		// Generate a single private/public key pair
		ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())
		// Create a mockSigner (bad name) that can sign using the previously generated key
		mockSigner := types.NewMockSigner(ki)
		// Generate SignedMessages
		newSignedMessage := types.NewSignedMessageForTestGetter(mockSigner)
		message := newSignedMessage()

		retVal := []byte{1, 2, 3}
		receipt := &types.MessageReceipt{
			ExitCode: 123,
			Return:   []types.Bytes{retVal},
		}
		child.Messages = []*types.SignedMessage{message}
		child.MessageReceipts = []*types.MessageReceipt{receipt}

		marshaled, e1 := json.Marshal(child)
		assert.NoError(e1)
		str := string(marshaled)

		assert.Contains(str, parent.Cid().String())
		assert.Contains(str, message.From.String())
		assert.Contains(str, message.To.String())

		// marshal/unmarshal symmetry
		var unmarshalled types.Block
		e2 := json.Unmarshal(marshaled, &unmarshalled)
		assert.NoError(e2)

		assert.Equal(uint8(123), unmarshalled.MessageReceipts[0].ExitCode)
		assert.Equal([]types.Bytes{[]byte{1, 2, 3}}, unmarshalled.MessageReceipts[0].Return)

		types.AssertHaveSameCid(assert, child, &unmarshalled)
	})
}