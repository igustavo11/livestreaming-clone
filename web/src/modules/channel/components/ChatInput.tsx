import { type FormEvent, useState } from "react";
import { Input } from "@/components/ui/input";

const MAX_MESSAGE_LEN = 200;

type ChatInputProps = {
	disabled: boolean;
	onSend: (message: string) => void;
};

export function ChatInput({ disabled, onSend }: ChatInputProps) {
	const [value, setValue] = useState("");

	function handleSubmit(event: FormEvent) {
		event.preventDefault();
		const trimmed = value.trim();
		if (!trimmed || trimmed.length > MAX_MESSAGE_LEN) return;
		onSend(trimmed);
		setValue("");
	}

	return (
		<form onSubmit={handleSubmit}>
			<Input
				value={value}
				onChange={(event) => setValue(event.target.value)}
				disabled={disabled}
				maxLength={MAX_MESSAGE_LEN}
				placeholder={
					disabled ? "Faça login para conversar" : "Enviar uma mensagem"
				}
			/>
		</form>
	);
}
