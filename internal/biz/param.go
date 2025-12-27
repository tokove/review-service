package biz

// 运营端审核评价的参数
type AuditParam struct {
	ReviewID  int64
	Status    int32
	OpUser    string
	OpReason  string
	OpRemarks string
}

// 商家端回复评价的参数
type ReplyParam struct {
	ReviewID  int64
	StoreID   int64
	Content   string
	PicInfo   string
	VideoInfo string
}
