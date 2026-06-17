/**
 * Auth Service - Authentication, account management, and character operations
 */
import { WorldSocket, OpCodes } from "@/net";
import { WT_IP, WT_PORT } from "@/config";
import { getExistingGuestToken } from "@/utils/guestToken";

// JWT Response interface
export interface JWTResponse {
    status: number;
    message: string;
    token?: string;
}

/**
 * Login with email and password
 */
export async function login(email: string, password: string): Promise<JWTResponse> {
    if (!WorldSocket.isConnected) {
        const connected = await WorldSocket.connect(WT_IP, WT_PORT, () => { });
        if (!connected) throw new Error("Failed to connect to server");
    }

    const response = await WorldSocket.sendJsonRequest(
        OpCodes.JWTLogin,
        OpCodes.JWTResponse,
        { token: `${email}:${password}` },
        5000
    ) as JWTResponse;

    if (response.status <= 0) {
        throw new Error(response.message || "Invalid email or password");
    }

    return response;
}

/**
 * Login as guest with a token
 */
export async function loginGuest(token: string): Promise<JWTResponse> {
    if (!WorldSocket.isConnected) {
        const connected = await WorldSocket.connect(WT_IP, WT_PORT, () => { });
        if (!connected) throw new Error("Failed to connect to server");
    }

    const response = await WorldSocket.sendJsonRequest(
        OpCodes.JWTLogin,
        OpCodes.JWTResponse,
        { token: `guest:${token}` },
        5000
    ) as JWTResponse;

    if (response.status <= 0) {
        throw new Error(response.message || "Authentication failed");
    }

    return response;
}

/**
 * Register a new account
 */
export async function registerAccount(email: string, password: string): Promise<JWTResponse> {
    if (!WorldSocket.isConnected) {
        const connected = await WorldSocket.connect(WT_IP, WT_PORT, () => { });
        if (!connected) throw new Error("Failed to connect to server");
    }

    const guestToken = getExistingGuestToken();
    const request: { token: string; guestToken?: string } = {
        token: `register:${email}:${password}`,
    };
    if (guestToken) {
        request.guestToken = guestToken;
    }

    const response = await WorldSocket.sendJsonRequest(
        OpCodes.JWTLogin,
        OpCodes.JWTResponse,
        request,
        5000
    ) as JWTResponse;

    if (response.status <= 0) {
        throw new Error(response.message || "Registration failed");
    }

    return response;
}

/**
 * Create a new character
 */
export async function createCharacter(characterData: any): Promise<boolean> {
    if (!WorldSocket.isConnected) {
        throw new Error("WorldSocket not connected");
    }

    const response = await WorldSocket.sendJsonRequest(
        OpCodes.CharacterCreate,
        OpCodes.CharacterCreateResponse,
        characterData
    ) as { value: number };

    return response.value === 1;
}

/**
 * Enter the world with a character
 */
export async function enterWorld(name: string): Promise<boolean> {
    if (!WorldSocket.isConnected) {
        throw new Error("WorldSocket not connected");
    }

    const response = await WorldSocket.sendJsonRequest(
        OpCodes.EnterWorld,
        OpCodes.PostEnterWorld,
        {
            name: name,
            tutorial: 0,
            returnHome: 0,
        }
    ) as { value: number };

    return response.value === 1;
}

/**
 * Delete a character
 */
export async function deleteCharacter(name: string): Promise<boolean> {
    if (!WorldSocket.isConnected) {
        throw new Error("WorldSocket not connected");
    }

    await WorldSocket.sendJsonMessage(OpCodes.DeleteCharacter, {
        value: name,
    });

    return true;
}

/**
 * Validate a character name
 */
export async function validateName(name: string): Promise<{
    valid: boolean;
    available: boolean;
    errorMessage: string;
} | null> {
    try {
        if (!WorldSocket.isConnected) {
            console.warn("WorldSocket not connected for validateName");
            return null;
        }
        const response = await WorldSocket.sendJsonRequest(
            OpCodes.ValidateNameRequest,
            OpCodes.ValidateNameResponse,
            { name }
        ) as any;
        return {
            valid: !!(response.valid || response.success),
            available: !!response.available,
            errorMessage: response.errorMessage || "",
            ...response,
        };
    } catch (error) {
        console.error("Error validating name via JSON:", error);
        return null;
    }
}
