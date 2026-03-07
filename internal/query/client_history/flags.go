package clienthistory

type ClientHistoryFlag string

const (
	FlagPaymentRequired ClientHistoryFlag = "payment_required"
	FlagAttention       ClientHistoryFlag = "attention"
	FlagReliable        ClientHistoryFlag = "reliable"
)
