package link

var (
	ReportError func(err error)
	ReportData  func(callbackType int, data []byte)
)
