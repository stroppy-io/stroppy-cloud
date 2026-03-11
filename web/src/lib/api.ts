import { createClient } from "@connectrpc/connect"
import { createConnectTransport } from "@connectrpc/connect-web"
import { StroppyAPI } from "@/proto/api/api_pb.ts"

const transport = createConnectTransport({
  baseUrl: window.location.origin,
  interceptors: [
    (next) => async (req) => {
      const token = localStorage.getItem("access_token")
      if (token) {
        req.header.set("Authorization", `Bearer ${token}`)
      }
      return next(req)
    },
  ],
})

export const api = createClient(StroppyAPI, transport)
