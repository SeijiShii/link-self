interface MessageInputProps {
  value: string;
  onChange: (value: string) => void;
  onSend: () => void;
  onKeyPress: (e: React.KeyboardEvent<HTMLInputElement>) => void;
}

export default function MessageInput({ value, onChange, onSend, onKeyPress }: MessageInputProps) {
  return (
    <div className="message-input">
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyPress={onKeyPress}
        placeholder="メッセージを入力..."
        className="message-input-field"
      />
      <button
        onClick={onSend}
        disabled={!value.trim()}
        className="message-send-button"
      >
        送信
      </button>
    </div>
  );
}
