const GUEST_TOKEN_KEY = "capturequest_guest_token";

export const getExistingGuestToken = (): string => {
  try {
    return localStorage.getItem(GUEST_TOKEN_KEY) || "";
  } catch {
    return "";
  }
};

export const getOrCreateGuestToken = (): string => {
  try {
    let token = localStorage.getItem(GUEST_TOKEN_KEY);
    if (!token) {
      if (
        typeof crypto !== "undefined" &&
        typeof crypto.randomUUID === "function"
      ) {
        token = crypto.randomUUID();
      } else {
        token =
          "guest_" +
          Math.random().toString(36).substring(2, 15) +
          Math.random().toString(36).substring(2, 15);
      }
      localStorage.setItem(GUEST_TOKEN_KEY, token);
    }
    return token;
  } catch (e) {
    console.warn(
      "[Login] Local storage access failed, generating temporary token:",
      e,
    );
    return "tmp_" + Math.random().toString(36).substring(2, 15);
  }
};
