package models

import (
	"errors"
	"strings"
	"time"

	"github.com/nanami9426/imgo/internal/utils"
	"gorm.io/gorm"
)

type LLMConversation struct {
	ConversationID     int64     `gorm:"primarykey"`
	UserID             int64     `gorm:"index"`
	Title              string    // 会话标题（默认截取首条 user 消息）
	Model              string    // 最近一次使用的模型
	MessageCount       int       // 会话总消息数（user/system/assistant）
	LastMessagePreview string    // 最近一条消息预览
	LastMessageAt      time.Time `gorm:"index"`
	Basic
}

func (c *LLMConversation) TableName() string {
	return "llm_conversation"
}

type LLMConversationMessage struct {
	MessageID      int64  `gorm:"primarykey"`
	ConversationID int64  `gorm:"index"`
	UserID         int64  `gorm:"index"`
	Role           string // system/user/assistant
	Content        string `gorm:"type:longtext"` // 文本内容（便于直接展示）
	MessageJSON    string `gorm:"type:longtext"` // 原始消息JSON（便于还原转发）
	Model          string
	Basic
}

func (m *LLMConversationMessage) TableName() string {
	return "llm_conversation_message"
}

func CreateLLMConversation(conversation *LLMConversation) error {
	return utils.DB.Create(conversation).Error
}

// GetLLMConversationByIDAndUser 用于会话详情查询和归属校验。
func GetLLMConversationByIDAndUser(conversationID int64, userID int64) (*LLMConversation, error) {
	var conversation LLMConversation
	err := utils.DB.
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		First(&conversation).Error
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

// ConversationBelongsToUser 用于拦截跨用户访问会话。
func ConversationBelongsToUser(conversationID int64, userID int64) (bool, error) {
	var count int64
	err := utils.DB.
		Model(&LLMConversation{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Count(&count).Error
	return count > 0, err
}

// CountLLMConversationsByUser + ListLLMConversationsByUser 组成分页查询。
func CountLLMConversationsByUser(userID int64) (int64, error) {
	var count int64
	err := utils.DB.
		Model(&LLMConversation{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}

func ListLLMConversationsByUser(userID int64, offset int, limit int) ([]*LLMConversation, error) {
	var list []*LLMConversation
	err := utils.DB.
		Where("user_id = ?", userID).
		Order("last_message_at DESC").
		Order("conversation_id DESC").
		Offset(offset).
		Limit(limit).
		Find(&list).Error
	return list, err
}

func CreateLLMConversationMessages(messages []*LLMConversationMessage) error {
	if len(messages) == 0 {
		return nil
	}
	return utils.DB.Create(messages).Error
}

// CountLLMConversationMessages + ListLLMConversationMessages 用于消息分页查询。
func CountLLMConversationMessages(conversationID int64) (int64, error) {
	var count int64
	err := utils.DB.
		Model(&LLMConversationMessage{}).
		Where("conversation_id = ?", conversationID).
		Count(&count).Error
	return count, err
}

func ListLLMConversationMessages(conversationID int64, offset int, limit int) ([]*LLMConversationMessage, error) {
	var list []*LLMConversationMessage
	err := utils.DB.
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Order("message_id ASC").
		Offset(offset).
		Limit(limit).
		Find(&list).Error
	return list, err
}

func GetRecentLLMConversationMessages(conversationID int64, limit int) ([]*LLMConversationMessage, error) {
	var list []*LLMConversationMessage
	db := utils.DB.
		Where("conversation_id = ?", conversationID).
		Order("created_at DESC").
		Order("message_id DESC")
	if limit > 0 {
		db = db.Limit(limit)
	}
	if err := db.Find(&list).Error; err != nil {
		return nil, err
	}
	// 查询时按倒序拿最新 limit 条，这里反转成时间正序方便拼接上下文。
	reverseMessages(list)
	return list, nil
}

// RefreshLLMConversationStats 在每次写消息后刷新会话统计与预览字段。
func RefreshLLMConversationStats(conversationID int64, model string) error {
	count, err := CountLLMConversationMessages(conversationID)
	if err != nil {
		return err
	}

	var last LLMConversationMessage
	err = utils.DB.
		Where("conversation_id = ?", conversationID).
		Order("created_at DESC").
		Order("message_id DESC").
		First(&last).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	updates := map[string]interface{}{
		"message_count": int(count),
	}
	if strings.TrimSpace(model) != "" {
		updates["model"] = strings.TrimSpace(model)
	}
	if err == nil {
		updates["last_message_preview"] = truncateRunes(strings.TrimSpace(last.Content), 120)
		updates["last_message_at"] = last.CreatedAt
	}
	return utils.DB.
		Model(&LLMConversation{}).
		Where("conversation_id = ?", conversationID).
		Updates(updates).Error
}

func reverseMessages(messages []*LLMConversationMessage) {
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
}

// truncateRunes 按字符截断，避免中文被按字节切坏。
func truncateRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= limit {
		return s
	}
	return string(rs[:limit])
}
