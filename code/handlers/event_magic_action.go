package handlers

import (
	"fmt"

	"start-feishubot/utils"
	// larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type MagicHFAction struct { /*HuggingFace模式*/
}

func (*MagicHFAction) Execute(a *ActionInfo) bool {
	if msg, foundMode := utils.EitherCutPrefix(a.info.qParsed,
		"/magic ", "魔法模式 "); foundMode {
		a.handler.sessionCache.Clear(*a.info.sessionId)

		bs64, err := a.handler.hf.GenerateImage(msg)
		if err != nil {
			replyMsg(*a.ctx, fmt.Sprintf(
				"🤖️：魔法施展失败，请稍后再试～\n错误信息: %v", err), a.info.msgId)
			return false
		}
		replayImageCardByBase64(*a.ctx, bs64, a.info.msgId, a.info.sessionId,
			a.info.qParsed)
		return false
	}
	return true
}
