package session

// MetadataKeyOutboundRecipientUserID is set on bus.InboundMessage by the send_message tool when
// targeting bus.Recipient.UserID; Engine.SendMessage copies it onto OutboundMessage.To.UserID.
const MetadataKeyOutboundRecipientUserID = "oneclaw.outbound_to_user_id"
