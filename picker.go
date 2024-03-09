package gobt

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/edwces/gobt/bitfield"
	"golang.org/x/exp/slices"
)

const (
	DefaultBlockSize      = 16000
	RandomPieceEndCounter = 5
)

func PieceCount(tSize, pMaxSize int) int {
	return int(math.Ceil((float64(tSize) / float64(pMaxSize))))
}

func BlockCount(tSize, pMaxSize, pIndex int) int {
	pSize := math.Min(float64(pMaxSize), float64(tSize)-float64(pMaxSize)*float64(pIndex))
	return int(math.Ceil((float64(pSize) / float64(DefaultBlockSize))))
}

type Picker struct {
	pCounter int
	tSize    int
	pMaxSize int

	manager *PieceManager
	ordered []int
	rand    *rand.Rand

	sync.Mutex
}

// NewPicker creates picker with pieces to pick from.
func NewPicker(tSize int, pMaxSize int, pm *PieceManager) *Picker {
	count := PieceCount(tSize, pMaxSize)
	ordered := make([]int, count)

	for i := 0; i < count; i++ {
		ordered[i] = i
	}

	rand := rand.New(rand.NewSource(time.Now().Unix()))

	return &Picker{tSize: tSize,
		pMaxSize: pMaxSize,
		manager:  pm,
		ordered:  ordered,
		rand:     rand}
}

func (p *Picker) SetRandSeed(seed int64) {
	p.rand.Seed(seed)
}

// Pick gets a new block from pieces that are available in bitfield.
func (p *Picker) Pick(have bitfield.Bitfield, peer string) (int, int, error) {
	p.Lock()
	defer p.Unlock()

	if len(p.ordered) == 0 {
		return p.pickEndgame(have, peer)
	}

	pIndex, error := p.pickPiece(have)

	if error != nil {
		return 0, 0, error
	}

	bIndex := p.pickBlock(pIndex, peer)

	return pIndex, bIndex, nil
}

func (p *Picker) IncrementPieceAvailability(pIndex int) {
	p.Lock()
	defer p.Unlock()

	state := p.manager.PieceAt(pIndex)
	state.availability++

	p.orderPieces()
}

func (p *Picker) DecrementAvailability(have bitfield.Bitfield) {
	p.Lock()
	defer p.Unlock()

	// TEMP Workaround, Should probably use some built-in func in bitfield
	count := PieceCount(p.tSize, p.pMaxSize)
	temp := make([]int, count)

	for i := range temp {
		if has, _ := have.Get(i); has {
			state := p.manager.PieceAt(i)
			state.availability--
		}
	}

	p.orderPieces()
}

func (p *Picker) IncrementAvailability(have bitfield.Bitfield) {
	p.Lock()
	defer p.Unlock()

	//  TEMP Workaround, Should probably use some built-in func in bitfield
	count := PieceCount(p.tSize, p.pMaxSize)
	temp := make([]int, count)

	for i := range temp {
		if has, _ := have.Get(i); has {
			state := p.manager.PieceAt(i)
			state.availability++
		}
	}

	p.orderPieces()
}

func (p *Picker) IsBlockResolving(pIndex int, bIndex int) bool {
	p.Lock()
	defer p.Unlock()

	state := p.manager.PieceAt(pIndex)
	return len(state.blocks[bIndex].peers) != 0
}

func (p *Picker) IsPieceDone(pIndex int) bool {
	p.Lock()
	defer p.Unlock()

	state := p.manager.PieceAt(pIndex)
	return state.status == PieceDone
}

// MarkPieceInQueue clears piece state and readds it to the picker
// NOTE: This method is unoptimized as it may cause loop where
//
//	the same peer/peers is constantly corrupting piece
func (p *Picker) MarkPieceInQueue(pIndex int) {
	p.Lock()
	defer p.Unlock()

	p.manager.ResetPiece(pIndex)

	p.ordered = append(p.ordered, pIndex)
	p.orderPieces()
}

// MarkBlockInQueue adds block to requests and optionally puts incomplete piece onto the top of picker
func (p *Picker) MarkBlockInQueue(pIndex int, bIndex int, peer string) {
	p.Lock()
	defer p.Unlock()

	state := p.manager.PieceAt(pIndex)
	state.blocks[bIndex].status = BlockInQueue
	state.blocks[bIndex].peers = slices.DeleteFunc(state.blocks[bIndex].peers, func(e string) bool { return e == peer })

	if state.status == PiecePending {
		state.status = PieceInProgress
		p.ordered = append(p.ordered, pIndex)
		p.orderPieces()
	}
}

// pickPiece returns and removes piece that is available in peer bitfield from picker ordered pieces.
func (p *Picker) pickPiece(have bitfield.Bitfield) (int, error) {
	reqBoundary := 0
	for i, val := range p.ordered {
		state := p.manager.PieceAt(val)

		if state.status == PieceInQueue {
			reqBoundary = i
			break
		}

		if have, _ := have.Get(val); have {
			return val, nil
		}
	}

	if p.pCounter < RandomPieceEndCounter {
		return p.pickRandomPiece(have, reqBoundary)
	}

	return p.pickRarestPiece(have, reqBoundary)
}

func (p *Picker) pickEndgame(have bitfield.Bitfield, peer string) (int, int, error) {
	for pIndex, state := range p.manager.GetPieces() {
		if state.status == PiecePending {
			has, _ := have.Get(pIndex)
			if !has {
				continue
			}

			bIndex, err := p.pickBlockEndgame(pIndex, peer)
			if err != nil {
				continue
			}

			state.blocks[bIndex].peers = append(state.blocks[bIndex].peers, peer)
			return pIndex, bIndex, nil
		}
	}

	return 0, 0, errors.New("No piece found")
}

func (p *Picker) pickBlockEndgame(pIndex int, peer string) (int, error) {
	state := p.manager.PieceAt(pIndex)

	for bIndex, block := range state.blocks {
		if block.status == BlockPending && !slices.Contains(block.peers, peer) {
			return bIndex, nil
		}
	}

	return 0, errors.New("No blocks found")
}

func (p *Picker) pickRandomPiece(have bitfield.Bitfield, reqBoundary int) (int, error) {
	ordCopy := make([]int, len(p.ordered)-reqBoundary)
	copy(ordCopy, p.ordered[reqBoundary:])
	p.rand.Shuffle(len(ordCopy), func(i, j int) {
		ordCopy[i], ordCopy[j] = ordCopy[j], ordCopy[i]
	})

	for _, val := range ordCopy {
		if have, _ := have.Get(val); have {
			return val, nil
		}
	}

	return 0, errors.New("No piece found")
}

func (p *Picker) pickRarestPiece(have bitfield.Bitfield, reqBoundary int) (int, error) {
	for _, val := range p.ordered[reqBoundary:] {
		if have, _ := have.Get(val); have {
			return val, nil
		}
	}

	return 0, errors.New("No piece found")
}

// removePiece returns true if succesfully removes piece from picker
func (p *Picker) removePiece(pIndex int) bool {
	for i, val := range p.ordered {
		if pIndex == val {
			p.ordered = append(p.ordered[:i], p.ordered[i+1:]...)
			return true
		}
	}
	return false
}

// pickBlock returns block index and removes piece from picker if all blocks have been requested
func (p *Picker) pickBlock(pIndex int, peer string) int {
	state := p.manager.PieceAt(pIndex)
	var bIndex int

	for bi, block := range state.blocks {
		if block.status == BlockInQueue {
			bIndex = bi
			block.status = BlockPending
			block.peers = append(block.peers, peer)
			break
		}
	}

	// TEMP:
	isPieceResolving := true

	for _, block := range state.blocks {
		if block.status == BlockInQueue {
			isPieceResolving = false
			break
		}
	}

	if isPieceResolving {
		p.removePiece(pIndex)
		state.status = PiecePending
		return bIndex
	}

	if state.status == PieceInQueue {
		state.status = PieceInProgress
		p.pCounter++
		p.orderPieces()
	}

	return bIndex
}

// Returns piece state or creates one if it doesn't exists
func (p *Picker) createState(pIndex int) *Piece {
	bCount := BlockCount(p.tSize, p.pMaxSize, pIndex)
	blocks := make([]*Block, bCount)
	for i := 0; i < bCount; i++ {
		block := &Block{status: BlockInQueue}
		blocks[i] = block
	}

	return &Piece{blocks: blocks, status: PieceInQueue}
}

func (p *Picker) orderPieces() {
	slices.SortFunc(p.ordered, func(a, b int) int {
		aState := p.manager.PieceAt(a)
		bState := p.manager.PieceAt(b)

		if aState.status == PieceInProgress && bState.status == PieceInQueue {
			return -1
		} else if bState.status == PieceInProgress && aState.status == PieceInQueue {
			return 1
		} else if aState.status == bState.status {
			// Sort based on availability
			if aState.availability < bState.availability {
				return -1
			} else if bState.availability < aState.availability {
				return 1
			} else {
				return 0
			}
		}

		return 0
	})
}
