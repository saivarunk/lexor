import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Preferences } from "../services/api";

export function useAppearance() {
  const client = useQueryClient();
  const preferences = useQuery({
    queryKey: ["preferences"],
    queryFn: () => api<Preferences>("/settings"),
  });
  const appearance = useMutation({
    mutationKey: ["appearance"],
    scope: { id: "appearance" },
    mutationFn: async (theme: "dark" | "light") => {
      const current = await api<Preferences>("/settings");
      return api<Preferences>("/settings", {
        method: "PUT",
        body: JSON.stringify({ ...current, theme, editorTheme: theme }),
      });
    },
    onSuccess: (value) => client.setQueryData(["preferences"], value),
  });
  const theme =
    preferences.data?.theme ?? localStorage.getItem("lexor:theme") ?? "dark";
  return { preferences, appearance, theme };
}
