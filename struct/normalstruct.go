package __

type SendMsg struct {
	ID   int
	Type int
	Msg  []byte
}

type RBCOut struct {
	Value []byte
	ID    int
}
