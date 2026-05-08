package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	v1 "review-service/api/review/v1"
	"review-service/internal/biz"
	"review-service/internal/data/model"
	"review-service/internal/data/query"
	"review-service/pkg/snowflake"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type reviewRepo struct {
	data *Data
	log  *log.Helper
}

// NewGreeterRepo .
func NewReviewRepo(data *Data, logger log.Logger) biz.ReviewRepo {
	return &reviewRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// SaveReview 将评价存入数据库
func (r *reviewRepo) SaveReview(ctx context.Context, review *model.ReviewInfo) (*model.ReviewInfo, error) {
	err := r.data.query.ReviewInfo.WithContext(ctx).Save(review)
	return review, err
}

// ExistsReviewByOrderID 根据订单ID查询数据库中是否存在评价
func (r *reviewRepo) ExistsReviewByOrderID(ctx context.Context, orderID int64) (bool, error) {
	cnt, err := r.data.query.ReviewInfo.WithContext(ctx).Where(r.data.query.ReviewInfo.OrderID.Eq(orderID)).Count()
	return cnt > 0, err
}

// GetReview 根据 reviewID 获取评价
func (r *reviewRepo) GetReview(ctx context.Context, reviewID int64) (*model.ReviewInfo, error) {
	return r.data.query.ReviewInfo.WithContext(ctx).Where(r.data.query.ReviewInfo.ReviewID.Eq(reviewID)).First()
}

// AuditReview 根据审核更新相应的评价
func (r *reviewRepo) AuditReview(ctx context.Context, param *biz.AuditParam) error {
	_, err := r.data.query.ReviewInfo.WithContext(ctx).
		Where(r.data.query.ReviewInfo.ReviewID.Eq(param.ReviewID)).
		Updates(map[string]interface{}{
			"status":     param.Status,
			"op_user":    param.OpUser,
			"op_reason":  param.OpReason,
			"op_remarks": param.OpRemarks,
		})
	return err
}

// SaveReply 将回复存入数据库
func (r *reviewRepo) SaveReply(ctx context.Context, reply *model.ReviewReplyInfo) (*model.ReviewReplyInfo, error) {
	// 数据校验（已回复的不能再回复）
	review, err := r.data.query.ReviewInfo.WithContext(ctx).
		Where(r.data.query.ReviewInfo.ReviewID.Eq(reply.ReviewID)).First()
	if err != nil {
		return nil, err
	}
	if review.HasReply == 1 {
		return nil, v1.ErrorOrderReplied("该评价已回复")
	}
	// 水平越权（商家只能回复自己店铺的评论）
	if review.StoreID != reply.StoreID {
		return nil, v1.ErrorStorePermissionDenied("水平越权")
	}
	// 存储到数据库（评价表和评价回复表）
	// 事务操作
	err = r.data.query.Transaction(func(tx *query.Query) error {
		// 评价表更新
		if _, err := tx.ReviewInfo.WithContext(ctx).
			Where(tx.ReviewInfo.ReviewID.Eq(review.ReviewID)).
			Update(tx.ReviewInfo.HasReply, 1); err != nil {
			r.log.WithContext(ctx).Errorf("update review failed, err:%v", err)
			return err
		}
		// 回复表保存
		if err := tx.ReviewReplyInfo.WithContext(ctx).Save(reply); err != nil {
			r.log.WithContext(ctx).Errorf("save reply failed, err:%v", err)
			return err
		}
		return nil
	})
	return reply, err
}

// AppealReview 申诉评价
func (r *reviewRepo) AppealReview(ctx context.Context, param *biz.AppealParam) (*model.ReviewAppealInfo, error) {
	ret, err := r.data.query.ReviewAppealInfo.WithContext(ctx).
		Where(
			r.data.query.ReviewAppealInfo.ReviewID.Eq(param.ReviewID),
			r.data.query.ReviewAppealInfo.StoreID.Eq(param.StoreID),
		).Take()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err == nil && ret.Status > 10 {
		return nil, errors.New("改评价已经有审核通过的申诉")
	}
	// 查询不到审核过的申诉记录
	// 1. 有申诉记录但是处于待审核状态，需要更新
	// if ret != nil{
	// 	// update
	// }else{
	// 	// insert
	// }
	// 2. 没有申诉记录，需要创建
	appeal := &model.ReviewAppealInfo{
		ReviewID:  param.ReviewID,
		StoreID:   param.StoreID,
		Status:    10,
		Reason:    param.Reason,
		Content:   param.Content,
		PicInfo:   param.PicInfo,
		VideoInfo: param.VideoInfo,
	}
	if ret != nil {
		appeal.AppealID = ret.AppealID
	} else {
		appeal.AppealID = snowflake.GenID()
	}
	err = r.data.query.ReviewAppealInfo.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "review_id"}, // ON DUPLICATE KEY
			},
			DoUpdates: clause.Assignments(map[string]interface{}{ // UPDATE
				"status":     appeal.Status,
				"content":    appeal.Content,
				"reason":     appeal.Reason,
				"pic_info":   appeal.PicInfo,
				"video_info": appeal.VideoInfo,
			}),
		}).
		Create(appeal) // INSERT
	r.log.Debugf("AppealReview, err:%v", err)
	return appeal, err
}

// AuditAppeal 审核申诉
func (r *reviewRepo) AuditAppeal(ctx context.Context, param *biz.AuditAppealParam) error {
	err := r.data.query.Transaction(func(tx *query.Query) error {
		if _, err := tx.ReviewAppealInfo.WithContext(ctx).
			Where(r.data.query.ReviewAppealInfo.AppealID.Eq(param.AppealID)).
			Updates(map[string]interface{}{
				"status":  param.Status,
				"op_user": param.OpUser,
				"reason":  param.OpReason,
			}); err != nil {
			return err
		}
		if param.Status == 20 {
			if _, err := tx.ReviewInfo.WithContext(ctx).
				Where(r.data.query.ReviewInfo.ReviewID.Eq(param.ReviewID)).
				Update(tx.ReviewInfo.Status, 40); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// ListReviewsByUserId 根据用户ID查询评价列表
func (r *reviewRepo) ListReviewsByUserId(ctx context.Context, param *biz.ListReviewsByUserIdParam) ([]*model.ReviewInfo, error) {
	return r.data.query.ReviewInfo.WithContext(ctx).
		Where(r.data.query.ReviewInfo.UserID.Eq(param.UserID)).
		Order(r.data.query.ReviewInfo.CreateAt.Desc()).
		Offset(int((param.Page - 1) * param.Size)).
		Limit(int(param.Size)).
		Find()
}

// ListReviewsByStoreId 根据商户ID查询评价列表
func (r *reviewRepo) ListReviewsByStoreId(ctx context.Context, param *biz.ListReviewsByStoreIdParam) ([]*biz.MyReviewInfo, error) {
	// return r.getData1(ctx, param) // 直接插es
	return r.getData2(ctx, param) // 先查redis，再查es
}

func (r *reviewRepo) getData1(ctx context.Context, param *biz.ListReviewsByStoreIdParam) ([]*biz.MyReviewInfo, error) {
	resp, err := r.data.es.
		Search().
		Index("review").
		From(int((param.Page - 1) * param.Size)).
		Size(int(param.Size)).
		Query(
			&types.Query{
				Bool: &types.BoolQuery{
					Filter: []types.Query{
						{
							Term: map[string]types.TermQuery{
								"store_id": {Value: param.StoreID},
							},
						},
					},
				},
			},
		).Do(ctx)
	if err != nil {
		return nil, err
	}
	reviews := make([]*biz.MyReviewInfo, 0, resp.Hits.Total.Value)
	for _, hit := range resp.Hits.Hits {
		var review biz.MyReviewInfo
		if err := json.Unmarshal(hit.Source_, &review); err != nil {
			r.log.WithContext(ctx).Errorf("failed to unmarshal review info, err:%v", err)
			continue
		}
		reviews = append(reviews, &review)
	}
	return reviews, nil
}

func (r *reviewRepo) getData2(ctx context.Context, param *biz.ListReviewsByStoreIdParam) ([]*biz.MyReviewInfo, error) {
	// 1. 从 redis 中查
	// 2. 从 es 中查
	// 3. 利用 singleflight
	key := fmt.Sprintf("review:%d:%d:%d", param.StoreID, param.Page, param.Size)
	data, err := r.getDataBySingleFlight(ctx, key)
	if err != nil {
		return nil, err
	}
	hits := new(types.HitsMetadata)
	if err := json.Unmarshal(data, hits); err != nil {
		return nil, err
	}
	reviews := make([]*biz.MyReviewInfo, 0, hits.Total.Value)
	for _, hit := range hits.Hits {
		var review biz.MyReviewInfo
		if err := json.Unmarshal(hit.Source_, &review); err != nil {
			r.log.WithContext(ctx).Errorf("failed to unmarshal review info, err:%v", err)
			continue
		}
		reviews = append(reviews, &review)
	}
	return reviews, nil
}

func (r *reviewRepo) getDataBySingleFlight(ctx context.Context, key string) ([]byte, error) {
	var g singleflight.Group
	data, err, _ := g.Do(key, func() (any, error) {
		data, err := r.getDataFromCache(ctx, key)
		r.log.Debugf("getDataBySingleFlight getDataFromCache, key: %v, data: %v, err: %v", key, data, err)
		if err == nil {
			r.log.Debugf("getDataFromCache hit, key: %v, value: %v", key, string(data))
			return data, nil
		}
		if errors.Is(err, redis.Nil) {
			data, err = r.getDataFromES(ctx, key)
			if err == nil {
				r.log.Debugf("setDataToCache, key: %v, data: %v", key, data)
				return data, r.setDataToCache(ctx, key, data)
			}
		}
		return nil, err
	})
	if err != nil {
		return nil, err
	}
	return data.([]byte), nil
}

func (r *reviewRepo) getDataFromCache(ctx context.Context, key string) ([]byte, error) {
	r.log.Debugf("getDataFromCache key: %v", key)
	return r.data.rdb.Get(ctx, key).Bytes()
}

func (r *reviewRepo) setDataToCache(ctx context.Context, key string, data []byte) error {
	r.log.Debugf("setDataToCache key: %v", key)
	return r.data.rdb.Set(ctx, key, data, 10*time.Second).Err()
}

// key: review:store_id:page:size
func (r *reviewRepo) getDataFromES(ctx context.Context, key string) ([]byte, error) {
	r.log.Debugf("getDataFromES key: %v", key)
	parts := strings.Split(key, ":")
	if len(parts) != 4 {
		return nil, errors.New("invalid key format")
	}
	storeID := parts[1]
	page, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return nil, err
	}
	size, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return nil, err
	}
	resp, err := r.data.es.
		Search().
		Index("review").
		From(int((page - 1) * size)).
		Size(int(size)).
		Query(
			&types.Query{
				Bool: &types.BoolQuery{
					Filter: []types.Query{
						{
							Term: map[string]types.TermQuery{
								"store_id": {Value: storeID},
							},
						},
					},
				},
			},
		).Do(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resp.Hits)
}
