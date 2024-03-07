package gobt

type PieceStatus int
type BlockStatus int

const (
	PieceInQueue PieceStatus = iota
	PieceInProgress
	PiecePending
	PieceDone

	BlockInQueue BlockStatus = iota
	BlockPending
	BlockDone
)

type Block struct {
	status BlockStatus
	peers  []string
}

type Piece struct {
	blocks       []*Block
	status       PieceStatus
	availability int
}

type PieceManager struct {
	tSize    int
	pMaxSize int

	pieces map[int]*Piece
}

func NewPieceManager(tSize, pMaxSize int) *PieceManager {
	return &PieceManager{pieces: map[int]*Piece{}, tSize: tSize, pMaxSize: pMaxSize}
}

// func (p *PieceManager) MarkBlockInQueue(pi, bi int) {
// 	piece := p.PieceAt(pi)
// 	piece.blocks[bi].status = BlockInQueue
//
// 	if piece.status == PiecePending {
// 		piece.status = PieceInProgress
// 	}
// }

//	func (p *PieceManager) MarkBlockPending(pi, bi int) {
//		piece := p.PieceAt(pi)
//		piece.blocks[bi].status = BlockPending
//
//		if piece.status == PieceInQueue {
//			piece.status = PieceInProgress
//		} else if piece.status == PieceInProgress {
//			for
//		}
//	}
func (p *PieceManager) ResetPiece(pi int) {
	p.pieces[pi] = p.createPiece(pi)
}

func (p *PieceManager) GetPieces() map[int]*Piece {
	return p.pieces
}

func (p *PieceManager) PieceAt(pi int) *Piece {
	piece, exists := p.pieces[pi]

	if !exists {
		piece = p.createPiece(pi)
		p.pieces[pi] = piece
	}

	return piece
}

func (p *PieceManager) createPiece(pi int) *Piece {
	blockCount := BlockCount(p.tSize, p.pMaxSize, pi)
	blocks := make([]*Block, blockCount)

	for j := 0; j < blockCount; j++ {
		block := &Block{status: BlockInQueue}
		blocks[j] = block
	}

	return &Piece{blocks: blocks, status: PieceInQueue}
}
