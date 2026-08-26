import { createSlice, type PayloadAction } from "@reduxjs/toolkit";
import type { AuthChannel, User } from "@/shared/schemas/channel";

export type Session = { user: User; channel: AuthChannel } | null;

type AuthState = {
	session: Session;
	status: "loading" | "resolved";
};

const initialState: AuthState = { session: null, status: "loading" };

const authSlice = createSlice({
	name: "auth",
	initialState,
	reducers: {
		sessionResolved(state, action: PayloadAction<Session>) {
			state.session = action.payload;
			state.status = "resolved";
		},
	},
});

export const { sessionResolved } = authSlice.actions;
export const authReducer = authSlice.reducer;
