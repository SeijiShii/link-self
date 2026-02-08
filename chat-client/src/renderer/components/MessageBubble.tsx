import type { Message } from '../types';

interface MessageBubbleProps {
  message: Message;
}

export default function MessageBubble({ message }: MessageBubbleProps) {
  const timeString = message.timestamp.toLocaleTimeString('ja-JP', {
    hour: '2-digit',
    minute: '2-digit',
  });

  return (
    <div className={`message-bubble ${message.isSent ? 'sent' : 'received'}`}>
      <div className="message-content">
        {message.text}
      </div>
      <div className="message-time">
        {timeString}
      </div>
    </div>
  );
}
