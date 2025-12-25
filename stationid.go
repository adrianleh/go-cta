package cta

const (
	TYPE_BUS_STATION   = "bus"
	TYPE_TRAIN_STATION = "train-station"
	TYPE_TRAIN_STOP    = "train-stop"
)

func GetStationIdType(id uint) string {
	switch {
	case id < 30000:
		return TYPE_BUS_STATION
	case id >= 30000 && id < 40000:
		return TYPE_TRAIN_STATION
	case id >= 40000 && id < 50000:
		return TYPE_TRAIN_STOP
	default:
		return "unknown"
	}
}
