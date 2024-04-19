package serialreceiver

type Serial struct {
	device string
}

func New(device string) (*Serial, error) {
	return &Serial{
		device: device,
	}, nil
}
