package speedingmvba

import (
	pb "jumbo/struct"
)

func (mvba *MVBA) gen_rawMsg(id int, round int, msgtype int, value []byte, loop int) pb.RawMsg {
	return pb.RawMsg{
		ID:     int32(id),
		Round:  int32(round),
		Type:   int32(msgtype),
		Values: value,
		Loop:   int32(loop),
	}

}
