package yukihubaccount

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// errEmptyImageURL 表示图片上传接口没有返回可用地址。
var errEmptyImageURL = errors.New("图片上传没有返回地址")

// 消息类型（与手机版一致）。
const (
	MsgTypeText  = "text"
	MsgTypeEmoji = "emoji"
	MsgTypeImage = "image"
)

// 在线状态。
const (
	PresenceOnline  = "online"
	PresenceOffline = "offline"
)

// Friend 是好友列表里的一项。
type Friend struct {
	ID        string `json:"id"`
	UID       int64  `json:"uid"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar,omitempty"`
	Signature string `json:"signature,omitempty"`
	// Status：online / away / offline（服务端按心跳时间判定，90s 内 online，
	// 90~300s away，超过 300s offline）
	Status string `json:"status,omitempty"`
	// Activity 是「正在玩：xxx」，对方关闭分享时为空
	Activity      string `json:"activity,omitempty"`
	Note          string `json:"note,omitempty"`
	LastMessage   string `json:"lastMessage,omitempty"`
	LastMessageAt string `json:"lastMessageAt,omitempty"`
	UnreadCount   int    `json:"unreadCount"`
}

// FriendRequest 是一条待处理的好友申请。
type FriendRequest struct {
	FriendshipID string `json:"friendshipId"`
	UID          int64  `json:"uid"`
	Nickname     string `json:"nickname"`
	Avatar       string `json:"avatar,omitempty"`
	Signature    string `json:"signature,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
}

// FriendList 是好友列表 + 待处理申请数。
type FriendList struct {
	Friends         []Friend        `json:"friends"`
	PendingRequests []FriendRequest `json:"pendingRequests,omitempty"`
	PendingCount    int             `json:"pendingCount"`
}

// ChatMessage 是一条聊天消息（私聊或群聊共用）。
type ChatMessage struct {
	ID           string `json:"id"`
	SenderID     string `json:"senderId,omitempty"`
	SenderUID    int64  `json:"senderUid"`
	SenderName   string `json:"senderName,omitempty"`
	SenderAvatar string `json:"senderAvatar,omitempty"`
	ReceiverID   string `json:"receiverId,omitempty"`
	GroupID      string `json:"groupId,omitempty"`
	Content      string `json:"content"`
	MsgType      string `json:"msgType"`
	CreatedAt    string `json:"createdAt,omitempty"`
	IsMine       bool   `json:"isMine"`
	ReplyToID    string `json:"replyToId,omitempty"`
	// 群聊里服务端可能会带上被回复的消息摘要
	ReplyPreview string `json:"replyPreview,omitempty"`
}

// ChatGroup 是一个群聊。
type ChatGroup struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
	MemberCount int    `json:"memberCount"`
	OnlineCount int    `json:"onlineCount"`
	UnreadCount int    `json:"unreadCount"`
}

// ChatEmoji 是一个本站表情。
type ChatEmoji struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ==================== 好友 ====================

// ListFriends 拉好友列表与待处理申请。
func (c *Client) ListFriends(ctx context.Context, token string) (FriendList, error) {
	body, err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/friends/list", token, nil)
	if err != nil {
		return FriendList{}, err
	}

	// 兼容 {friends:[...], pendingRequests:[...]} 与 {data:{...}} 两种外层
	payload := body
	if data, ok := body["data"].(map[string]any); ok {
		if _, hasFriends := data["friends"]; hasFriends {
			payload = data
		}
	}

	result := FriendList{}
	for _, raw := range toMapSlice(payload["friends"]) {
		result.Friends = append(result.Friends, parseFriend(raw))
	}
	for _, raw := range toMapSlice(payload["pendingRequests"]) {
		result.PendingRequests = append(result.PendingRequests, FriendRequest{
			FriendshipID: pickString(raw, "friendshipId", "friendship_id", "id"),
			UID:          pickInt64(raw, "uid", "userId", "userIdFrom"),
			Nickname:     pickString(raw, "nickname", "name"),
			Avatar:       pickString(raw, "avatarUrl", "avatar_url", "avatar"),
			Signature:    pickString(raw, "signature"),
			CreatedAt:    pickString(raw, "createdAt", "created_at"),
		})
	}
	// 待处理数：有的版本直接给 total，有的只给数组
	result.PendingCount = int(pickInt64(payload, "pendingCount", "totalPending", "requestCount"))
	if result.PendingCount == 0 && len(result.PendingRequests) > 0 {
		result.PendingCount = len(result.PendingRequests)
	}
	return result, nil
}

// SearchUsers 按关键词搜用户（返回的是好友列表同构的条目）。
func (c *Client) SearchUsers(ctx context.Context, token, keyword string) ([]Friend, error) {
	query := url.Values{"q": {strings.TrimSpace(keyword)}}
	body, err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/friends/search?"+query.Encode(), token, nil)
	if err != nil {
		return nil, err
	}
	raw := body["results"]
	if data, ok := body["data"].(map[string]any); ok {
		if _, has := data["results"]; has {
			raw = data["results"]
		}
	}
	results := make([]Friend, 0)
	for _, item := range toMapSlice(raw) {
		results = append(results, parseFriend(item))
	}
	return results, nil
}

// SendFriendRequest 发送好友申请。target 可以是 uid 或昵称。
func (c *Client) SendFriendRequest(ctx context.Context, token, target string) error {
	payload := map[string]string{"target": strings.TrimSpace(target)}
	return c.doEmpty(ctx, http.MethodPost, c.baseURL+"/friends/request", token, payload)
}

// AcceptFriendRequest 接受好友申请。
func (c *Client) AcceptFriendRequest(ctx context.Context, token, friendshipID string, uid int64) error {
	payload := map[string]any{}
	if strings.TrimSpace(friendshipID) != "" {
		payload["friendshipId"] = strings.TrimSpace(friendshipID)
	}
	if uid > 0 {
		payload["uid"] = uid
	}
	return c.doEmpty(ctx, http.MethodPost, c.baseURL+"/friends/accept", token, payload)
}

// RejectFriendRequest 拒绝好友申请。
func (c *Client) RejectFriendRequest(ctx context.Context, token, friendshipID string) error {
	payload := map[string]string{"friendshipId": strings.TrimSpace(friendshipID)}
	return c.doEmpty(ctx, http.MethodPost, c.baseURL+"/friends/reject", token, payload)
}

// RemoveFriend 删除好友。
func (c *Client) RemoveFriend(ctx context.Context, token, friendID string) error {
	payload := map[string]string{"friendId": strings.TrimSpace(friendID)}
	return c.doEmpty(ctx, http.MethodPost, c.baseURL+"/friends/remove", token, payload)
}

// SetFriendNote 设置好友备注。
func (c *Client) SetFriendNote(ctx context.Context, token, friendID, note string) error {
	payload := map[string]string{"friendId": strings.TrimSpace(friendID), "note": note}
	return c.doEmpty(ctx, http.MethodPost, c.baseURL+"/friends/note", token, payload)
}

// ==================== 私聊 ====================

// SendChatMessage 发私聊消息，返回落库后的消息（可能为空）。
func (c *Client) SendChatMessage(ctx context.Context, token, receiverID, content, msgType, replyToID string) (ChatMessage, error) {
	if strings.TrimSpace(msgType) == "" {
		msgType = MsgTypeText
	}
	payload := map[string]any{
		"receiverId": strings.TrimSpace(receiverID),
		"content":    content,
		"msgType":    msgType,
	}
	if strings.TrimSpace(replyToID) != "" {
		payload["replyToId"] = strings.TrimSpace(replyToID)
	}
	body, err := c.doJSON(ctx, http.MethodPost, c.baseURL+"/chat/send", token, payload)
	if err != nil {
		return ChatMessage{}, err
	}
	if message, ok := body["message"].(map[string]any); ok {
		return parseChatMessage(message), nil
	}
	return ChatMessage{}, nil
}

// ChatHistory 拉与某人的历史消息。
func (c *Client) ChatHistory(ctx context.Context, token, friendID string, offset, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		limit = 30
	}
	query := url.Values{
		"friendId": {strings.TrimSpace(friendID)},
		"offset":   {strconv.Itoa(offset)},
		"limit":    {strconv.Itoa(limit)},
	}
	body, err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/chat/history?"+query.Encode(), token, nil)
	if err != nil {
		return nil, err
	}
	return parseMessages(body["messages"]), nil
}

// ChatPoll 拉新消息（afterID 之后）。friendID 为空时拉所有会话的新消息。
func (c *Client) ChatPoll(ctx context.Context, token, afterID, friendID string, peek bool) ([]ChatMessage, error) {
	query := url.Values{"afterId": {strings.TrimSpace(afterID)}}
	if strings.TrimSpace(friendID) != "" {
		query.Set("friendId", strings.TrimSpace(friendID))
	}
	if peek {
		query.Set("peek", "1")
	}
	body, err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/chat/poll?"+query.Encode(), token, nil)
	if err != nil {
		return nil, err
	}
	return parseMessages(body["messages"]), nil
}

// UnreadTotal 返回未读消息总数。
func (c *Client) UnreadTotal(ctx context.Context, token string) (int, error) {
	body, err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/chat/unread", token, nil)
	if err != nil {
		return 0, err
	}
	return int(pickInt64(body, "totalUnread", "total", "count")), nil
}

// ListEmojis 拉本站表情列表。
func (c *Client) ListEmojis(ctx context.Context, token string) ([]ChatEmoji, error) {
	body, err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/chat/emojis", token, nil)
	if err != nil {
		return nil, err
	}
	emojis := make([]ChatEmoji, 0)
	for _, raw := range toMapSlice(body["emojis"]) {
		emojis = append(emojis, ChatEmoji{
			Name: pickString(raw, "name"),
			URL:  pickString(raw, "url"),
		})
	}
	return emojis, nil
}

// UploadChatImage 上传聊天图片，返回可直接放进消息 content 的地址。
func (c *Client) UploadChatImage(ctx context.Context, token, contentType string, data []byte) (string, error) {
	if strings.TrimSpace(contentType) == "" {
		contentType = "image/jpeg"
	}
	body, err := c.doRaw(ctx, http.MethodPost, c.baseURL+"/chat/upload_image", token, contentType, data)
	if err != nil {
		return "", err
	}
	imageURL := pickString(body, "url", "imageUrl", "image_url")
	if imageURL == "" {
		return "", errEmptyImageURL
	}
	return imageURL, nil
}

// ==================== 群聊 ====================

// ListGroups 拉群列表。
func (c *Client) ListGroups(ctx context.Context, token string) ([]ChatGroup, error) {
	body, err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/groups/list", token, nil)
	if err != nil {
		return nil, err
	}
	groups := make([]ChatGroup, 0)
	for _, raw := range toMapSlice(body["groups"]) {
		groups = append(groups, parseGroup(raw))
	}
	return groups, nil
}

// GroupHistory 拉群历史消息。
func (c *Client) GroupHistory(ctx context.Context, token, groupID string, offset, limit int) ([]ChatMessage, int, error) {
	if limit <= 0 {
		limit = 30
	}
	query := url.Values{
		"groupId": {strings.TrimSpace(groupID)},
		"offset":  {strconv.Itoa(offset)},
		"limit":   {strconv.Itoa(limit)},
	}
	body, err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/groups/messages?"+query.Encode(), token, nil)
	if err != nil {
		return nil, 0, err
	}
	return parseMessages(body["messages"]), int(pickInt64(body, "onlineCount")), nil
}

// SendGroupMessage 发群消息。
func (c *Client) SendGroupMessage(ctx context.Context, token, groupID, content, msgType, replyToID string) (ChatMessage, error) {
	if strings.TrimSpace(msgType) == "" {
		msgType = MsgTypeText
	}
	payload := map[string]any{
		"groupId": strings.TrimSpace(groupID),
		"content": content,
		"msgType": msgType,
	}
	if strings.TrimSpace(replyToID) != "" {
		payload["replyToId"] = strings.TrimSpace(replyToID)
	}
	body, err := c.doJSON(ctx, http.MethodPost, c.baseURL+"/groups/send", token, payload)
	if err != nil {
		return ChatMessage{}, err
	}
	if message, ok := body["message"].(map[string]any); ok {
		return parseChatMessage(message), nil
	}
	return ChatMessage{}, nil
}

// PollGroup 拉群新消息，同时返回当前在线人数。
func (c *Client) PollGroup(ctx context.Context, token, groupID, afterID string) ([]ChatMessage, int, error) {
	query := url.Values{
		"groupId": {strings.TrimSpace(groupID)},
		"afterId": {strings.TrimSpace(afterID)},
	}
	body, err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/groups/poll?"+query.Encode(), token, nil)
	if err != nil {
		return nil, 0, err
	}
	return parseMessages(body["messages"]), int(pickInt64(body, "onlineCount")), nil
}

// Action 是群消息管理动作（撤回 / 删除）。
type GroupMessageAction string

const (
	GroupActionRecall GroupMessageAction = "recall"
	GroupActionDelete GroupMessageAction = "delete"
)

// ManageGroupMessage 撤回或删除群消息。
func (c *Client) ManageGroupMessage(ctx context.Context, token, messageID string, action GroupMessageAction) error {
	payload := map[string]string{
		"messageId": strings.TrimSpace(messageID),
		"action":    string(action),
	}
	return c.doEmpty(ctx, http.MethodPost, c.baseURL+"/groups/manage", token, payload)
}

// ==================== 解析辅助 ====================

func parseFriend(raw map[string]any) Friend {
	friend := Friend{
		ID:            pickString(raw, "id", "friendId", "friend_id", "userId"),
		UID:           pickInt64(raw, "uid", "userId"),
		Nickname:      pickString(raw, "nickname", "name"),
		Avatar:        pickString(raw, "avatarUrl", "avatar_url", "avatar"),
		Signature:     pickString(raw, "signature"),
		Status:        pickString(raw, "status"),
		Activity:      pickString(raw, "activity"),
		Note:          pickString(raw, "note"),
		LastMessage:   pickString(raw, "lastMessage", "last_message"),
		LastMessageAt: pickString(raw, "lastMessageAt", "last_message_at", "updatedAt"),
		UnreadCount:   int(pickInt64(raw, "unreadCount", "unread")),
	}
	// 有些实现把 uid 放在嵌套的 user 里
	if friend.UID == 0 || friend.Nickname == "" {
		if nested, ok := raw["user"].(map[string]any); ok {
			if friend.UID == 0 {
				friend.UID = pickInt64(nested, "uid", "id")
			}
			if friend.Nickname == "" {
				friend.Nickname = pickString(nested, "nickname", "name")
			}
			if friend.Avatar == "" {
				friend.Avatar = pickString(nested, "avatarUrl", "avatar_url", "avatar")
			}
		}
	}
	return friend
}

func parseChatMessage(raw map[string]any) ChatMessage {
	message := ChatMessage{
		ID:           pickString(raw, "id", "messageId"),
		SenderID:     pickString(raw, "senderId", "sender_id"),
		SenderUID:    pickInt64(raw, "senderUid", "sender_uid", "uid"),
		SenderName:   pickString(raw, "senderName", "sender_name", "nickname"),
		SenderAvatar: pickString(raw, "senderAvatar", "sender_avatar", "avatarUrl", "avatar"),
		ReceiverID:   pickString(raw, "receiverId", "receiver_id"),
		GroupID:      pickString(raw, "groupId", "group_id"),
		Content:      pickString(raw, "content"),
		MsgType:      pickString(raw, "msgType", "msg_type", "type"),
		CreatedAt:    pickString(raw, "createdAt", "created_at", "time"),
		IsMine:       pickBool(raw, "isMine", "is_mine"),
		ReplyToID:    pickString(raw, "replyToId", "reply_to_id"),
		ReplyPreview: pickString(raw, "replyPreview", "reply_preview", "replyContent"),
	}
	if message.MsgType == "" {
		message.MsgType = MsgTypeText
	}
	return message
}

func parseGroup(raw map[string]any) ChatGroup {
	return ChatGroup{
		ID:          pickString(raw, "id", "groupId", "group_id"),
		Name:        pickString(raw, "name", "title"),
		Description: pickString(raw, "description", "intro"),
		Avatar:      pickString(raw, "avatarUrl", "avatar_url", "avatar"),
		MemberCount: int(pickInt64(raw, "memberCount", "member_count")),
		OnlineCount: int(pickInt64(raw, "onlineCount", "online_count")),
		UnreadCount: int(pickInt64(raw, "unreadCount", "unread")),
	}
}

func parseMessages(raw any) []ChatMessage {
	items := toMapSlice(raw)
	messages := make([]ChatMessage, 0, len(items))
	for _, item := range items {
		messages = append(messages, parseChatMessage(item))
	}
	return messages
}

// toMapSlice 把 any 归一成 []map[string]any，容忍 nil / 单对象 / 类型不符。
func toMapSlice(value any) []map[string]any {
	switch typed := value.(type) {
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if record, ok := item.(map[string]any); ok {
				result = append(result, record)
			}
		}
		return result
	case map[string]any:
		return []map[string]any{typed}
	default:
		return nil
	}
}
