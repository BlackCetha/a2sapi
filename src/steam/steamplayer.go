package steam

// steamplayer.go - steam server query for players (A2S_PLAYER)

import (
	"bytes"
	"encoding/binary"
	"math"
	"net"
	"steamtest/src/models"
	"steamtest/src/util"
	"sync"
	"time"
)

func getPlayerInfo(host string, timeout int) ([]byte, error) {
	conn, err := net.DialTimeout("udp", host, time.Duration(timeout)*time.Second)
	if err != nil {
		// TODO: simplify this log + return into just a return
		util.LogSteamError(ErrHostConnection(err.Error()))
		return nil, ErrHostConnection(err.Error())
	}

	defer conn.Close()
	conn.SetDeadline(time.Now().Add(time.Duration(timeout-1) * time.Second))

	_, err = conn.Write(playerChallengeReq)
	if err != nil {
		// TODO: simplify this log + return into just a return
		util.LogSteamError(ErrDataTransmit(err.Error()))
		return nil, ErrDataTransmit(err.Error())
	}

	challengeNumResp := make([]byte, maxPacketSize)
	_, err = conn.Read(challengeNumResp)
	if err != nil {
		// TODO: simplify this log + return into just a return
		util.LogSteamError(ErrDataTransmit(err.Error()))
		return nil, ErrDataTransmit(err.Error())
	}
	if !bytes.HasPrefix(challengeNumResp, expectedPlayerRespHeader) {
		// TODO: simplify this log + return into just a return
		util.LogSteamError(ErrChallengeResponse)
		return nil, ErrChallengeResponse
	}
	challengeNum := bytes.TrimLeft(challengeNumResp, headerStr)
	challengeNum = challengeNum[1:5]
	request := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x55}

	for _, b := range challengeNum {
		request = append(request, b)
	}

	_, err = conn.Write(request)
	if err != nil {
		// TODO: simplify this log + return into just a return
		util.LogSteamError(ErrDataTransmit(err.Error()))
		return nil, ErrDataTransmit(err.Error())
	}
	var buf [maxPacketSize]byte
	numread, err := conn.Read(buf[:maxPacketSize])
	if err != nil {
		// TODO: simplify this log + return into just a return
		util.LogSteamError(ErrDataTransmit(err.Error()))
		return nil, ErrDataTransmit(err.Error())
	}
	pi := make([]byte, numread)
	copy(pi, buf[:numread])

	return pi, nil
}

func parsePlayerInfo(unparsed []byte) ([]*models.SteamPlayerInfo, error) {
	if !bytes.HasPrefix(unparsed, expectedPlayerChunkHeader) {
		// TODO: simplify this log + return into just a return
		util.LogSteamError(ErrPacketHeader)
		return nil, ErrPacketHeader
	}
	unparsed = bytes.TrimLeft(unparsed, headerStr)
	numplayers := int(unparsed[1])

	if numplayers == 0 {
		return nil, ErrNoPlayers
	}

	players := []*models.SteamPlayerInfo{}

	// index 0 = '44' | 1 = 'numplayers' byte | 2 = player 1 separator byte '00'
	// | 3 = start of player 1 name; additional player start indexes are player separator + 1
	startidx := 3
	var b []byte
	for i := 0; i < numplayers; i++ {
		if i == 0 {
			b = unparsed[startidx:]
		} else {
			b = b[startidx+1:]
		}
		nul := bytes.IndexByte(b, 0x00)
		name := b[:nul]              // string (variable length)
		score := b[nul+1 : nul+5]    // long (4 bytes)
		duration := b[nul+5 : nul+9] // float (4 bytes)
		startidx = nul + 9

		seconds, timeformatted := getDuration(duration)
		players = append(players, &models.SteamPlayerInfo{
			Name:              string(name),
			Score:             int32(binary.LittleEndian.Uint32(score)),
			TimeConnectedSecs: seconds,
			TimeConnectedTot:  timeformatted,
		})
	}

	return players, nil
}

func getDuration(bytes []byte) (float32, string) {
	bits := binary.LittleEndian.Uint32(bytes)
	f := math.Float32frombits(bits)
	s := time.Duration(int64(f)) * time.Second
	return f, s.String()
}

func RetryFailedPlayersReq(failed []string,
	retrycount int) map[string][]*models.SteamPlayerInfo {

	m := make(map[string][]*models.SteamPlayerInfo)
	var f []string
	var wg sync.WaitGroup
	var mut sync.Mutex
	for i := 0; i < retrycount; i++ {
		if i == 0 {
			f = failed
		}
		wg.Add(len(f))
		for _, host := range f {
			go func(h string) {
				defer wg.Done()
				r, err := GetPlayersForServer(h, QueryTimeout)
				if err != nil {
					if err != ErrNoPlayers {
						return
					}
				}
				mut.Lock()
				m[h] = r
				f = removeFailedHost(f, h)
				mut.Unlock()
			}(host)
		}
		wg.Wait()
	}
	return m
}

func GetPlayersForServer(host string, timeout int) ([]*models.SteamPlayerInfo, error) {
	// Caller will log. Return err instead of wrapped util.LogSteamError so as not
	// to interfere with custom error types that need to be analyzed when
	// determining if retry needs to be done.
	pi, err := getPlayerInfo(host, timeout)
	if err != nil {
		return nil, err
	}

	players, err := parsePlayerInfo(pi)
	if err != nil {
		return nil, err
	}
	return players, nil
}