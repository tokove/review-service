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

// 商家端申诉评价的参数
type AppealParam struct {
	ReviewID  int64
	StoreID   int64
	Reason    string
	Content   string
	PicInfo   string
	VideoInfo string
}

// 运营端审核申诉评价的参数
type AuditAppealParam struct {
	AppealID  int64
	ReviewID  int64
	Status    int32
	OpUser    string
	OpReason  string
	OpRemarks string
}

// ListReviewsByUserIdParam 根据用户ID查询评价列表的参数
type ListReviewsByUserIdParam struct {
	UserID int64
	Page   int32
	Size   int32
}

// ListReviewsByStoreIdParam 根据商户ID查询评价列表的参数
type ListReviewsByStoreIdParam struct {
	StoreID int64
	Page    int32
	Size    int32
}
