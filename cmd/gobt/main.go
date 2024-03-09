package main

import (
	"fmt"
	"math"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/edwces/gobt"
	"github.com/edwces/gobt/bitfield"
	"github.com/edwces/gobt/message"
)

const (
	MaxPeerTimeout     = 2*time.Minute + 10*time.Second
	KeepAlivePeriod    = 1*time.Minute + 30*time.Second
	DefaultConnTimeout = 3 * time.Second
)

func main() {
	path := os.Args[1]

	// Open the metainfo file
	metainfo, err := gobt.Open(path)
	if err != nil {
		fmt.Println(err)
		return
	}

	clientID, err := gobt.GenRandPeerID()
	if err != nil {
		fmt.Println(err)
		return
	}

	hash, err := metainfo.Info.Hash()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Receive the peers from tracker
	peers, err := gobt.GetAvailablePeers(metainfo.Announce, hash, clientID, metainfo.Info.Length)
	if err != nil {
		fmt.Println(err)
		return
	}

	hashes, err := metainfo.Info.Hashes()
	if err != nil {
		fmt.Println(err)
		return
	}

	pm := gobt.NewPieceManager(metainfo.Info.Length, metainfo.Info.PieceLength)
	pp := gobt.NewPicker(metainfo.Info.Length, metainfo.Info.PieceLength, pm)
	storage := gobt.NewStorage(metainfo.Info.Length, metainfo.Info.PieceLength)
	clientBf := bitfield.New(len(hashes))
	connected := gobt.NewPeersManager()
	pCount := 0

	file, err := os.Create(metainfo.Info.Name)
	if err != nil {
		fmt.Println(err)
		return
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-c
		connected.Disconnect()
	}()

	var wg sync.WaitGroup

	for _, peer := range peers {
		wg.Add(1)
		go func(announcePeer gobt.AnnouncePeer) {
			conn, err := net.DialTimeout("tcp", announcePeer.Addr(), DefaultConnTimeout)
			if err != nil {
				fmt.Printf("connection error: %v\n", err)
				wg.Done()
				return
			}
			peer := gobt.NewPeer(conn)
			defer peer.Close()

			err = peer.Handshake(hash, clientID)
			if err != nil {
				fmt.Printf("handshake error: %v\n", err)
				wg.Done()
				return
			}

			connected.Add(peer)

			// Message loop
			bf := bitfield.New(len(hashes))

			defer func() {
				for _, req := range peer.Requests {
					pp.MarkBlockInQueue(req[0], req[1], peer.String())
				}
				pp.DecrementAvailability(bf)
				connected.Remove(peer)
				wg.Done()
			}()

			peer.SetWriteKeepAlive(KeepAlivePeriod)

			for {
				peer.SetReadKeepAlive(MaxPeerTimeout)
				msg, err := peer.ReadMsg()

				if err != nil {
					fmt.Println(err)
					return
				}

				if msg.KeepAlive {
					continue
				}

				switch msg.ID {
				case message.IDChoke:
					peer.IsChoking = true
				case message.IDPiece:
					block := msg.Payload.Block()

					err := peer.RecvRequest(int(block.Index), int(block.Offset), len(block.Block))
					if err != nil {
						fmt.Printf("invalid block received: %v\n", err)
						return
					}

					pm.MarkBlockDone(int(block.Index), int(block.Offset)/gobt.DefaultBlockSize, peer.String())
					if pp.IsBlockResolving(int(block.Index), int(block.Offset)/gobt.DefaultBlockSize) {
						connected.WriteCancel(int(block.Index), int(block.Offset), len(block.Block), peer.String())
					}

					// Store piece
					storage.SaveAt(int(block.Index), block.Block, int(block.Offset))

					if pp.IsPieceDone(int(block.Index)) {
						if storage.Verify(int(block.Index), hashes[block.Index]) {
							pCount++
							clientBf.Set(int(block.Index))
							fmt.Printf("-------------------------------------------------- %s GOT: %d; DONE: %d \n", announcePeer.Addr(), block.Index, pCount)
							file.WriteAt(storage.GetPieceData(int(block.Index)), int64(int(block.Index)*metainfo.Info.PieceLength))

							connected.WriteHave(int(block.Index), peer.String())

							if clientBf.Full() {
								connected.Disconnect()
							}
						} else {
							fmt.Printf("-------------------------------------------------- %s GOT FAILED: %d; \n", announcePeer.Addr(), block.Index)
							pp.MarkPieceInQueue(int(block.Index))
							peer.HashFails += 1
							if peer.HashFails >= gobt.MaxHashFails {
								fmt.Println("Excedded Maximum hash fails: 5")
								return
							}
						}
					}

					for peer.IsRequestable() {
						// Send request
						cp, cb, err := pp.Pick(bf, peer.String())

						if err != nil {
							err := peer.SendNotInterested()
							if err != nil {
								fmt.Println(err)
								return
							}
							break
						}

						length := int(math.Min(float64(gobt.DefaultBlockSize), float64(metainfo.Info.PieceLength)-float64(cb*gobt.DefaultBlockSize)))
						err = peer.SendRequest(cp, cb*gobt.DefaultBlockSize, length)
						if err != nil {
							fmt.Println(err)
							return
						}
					}

				case message.IDUnchoke:

					unresolved := [][]int{}
					if peer.IsChoking {
						unresolved = peer.Requests
						peer.Requests = [][]int{}
					}

					for peer.IsRequestable() {
						// Pick block to request
						var cp, cb int

						if len(unresolved) == 0 {
							cp, cb, err = pp.Pick(bf, peer.String())

							if err != nil {
								err := peer.SendNotInterested()
								if err != nil {
									fmt.Println(err)
									return
								}
								break
							}
						} else {
							cp = unresolved[0][0]
							cb = unresolved[0][1]
							unresolved = unresolved[1:]
						}

						length := int(math.Min(float64(gobt.DefaultBlockSize), float64(metainfo.Info.PieceLength)-float64(cb*gobt.DefaultBlockSize)))
						err = peer.SendRequest(cp, cb*gobt.DefaultBlockSize, length)
						if err != nil {
							fmt.Println(err)
							return
						}
					}

					peer.IsChoking = false
				case message.IDHave:
					have := int(msg.Payload.Have())
					err := bf.Set(have)

					if err != nil {
						fmt.Printf("Bitfield: %v\n", err)
						return
					}

					pp.IncrementPieceAvailability(have)

					if has, _ := clientBf.Get(have); !peer.IsInteresting && !has {
						err := peer.SendInterested()
						if err != nil {
							fmt.Println(err)
							return
						}
					}
				case message.IDBitfield:
					// Define peer bitfield
					err := bf.Replace(msg.Payload.Bitfield())
					if err != nil {
						fmt.Printf("Bitfield: %v\n", err)
						return
					}

					pp.IncrementAvailability(bf)

					// Calculate interesting pieces that peer has
					diff, err := bf.Difference(clientBf)
					if err != nil {
						fmt.Printf("Bitfield: %v\n", err)
						return
					}

					// Send interest status if it's not empty
					if !diff.Empty() {
						err := peer.SendInterested()
						if err != nil {
							fmt.Println(err)
							return
						}
					}
					// conn.WriteUnchoke()
				}
			}
		}(peer)
	}

	wg.Wait()
	file.Close()
	if !clientBf.Full() {
		os.Remove(metainfo.Info.Name)
	}
}
