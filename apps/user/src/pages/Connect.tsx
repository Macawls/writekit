import { useEffect, useState } from "react";
import { useStore } from "@nanostores/react";
import { type ClientInfo } from "../api";
import ClientIcon from "../components/ClientIcon";
import {
  $info, $clients, $error,
  loadConnect, connectClient as doConnect, disconnectClient as doDisconnect,
} from "../stores/connect";

export default function Connect() {
  const info = useStore($info);
  const clients = useStore($clients);
  const error = useStore($error);
  const [busy, setBusy] = useState<string | null>(null);
  const [justConnected, setJustConnected] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    loadConnect();
  }, []);

  const connect = async (c: ClientInfo) => {
    setBusy(c.id);
    try {
      await doConnect(c.id);
      setJustConnected(c.id);
      setTimeout(() => setJustConnected(null), 4000);
    } finally {
      setBusy(null);
    }
  };

  const disconnect = async (c: ClientInfo) => {
    setBusy(c.id);
    try {
      await doDisconnect(c.id);
    } finally {
      setBusy(null);
    }
  };

  const copy = () => {
    if (!info) return;
    navigator.clipboard.writeText(info.mcp_url).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };

  if (!info || !clients) {
    return (
      <>
        <h2>Connect</h2>
        <p className="muted">Loading...</p>
      </>
    );
  }

  const autoDetected = clients.filter((c) => c.detected && !c.manual);
  const manualClients = clients.filter((c) => c.manual);
  const undetected = clients.filter((c) => !c.detected && !c.manual);

  return (
    <>
      <h2>Connect</h2>
      <p className="muted" style={{ marginTop: ".25rem", maxWidth: 540 }}>
        Connect WriteKit to clients on your machine
      </p>

      <div className="connect-url" title="Only needed if a client asks for an MCP URL manually">
        <span className="connect-url-label">MCP URL</span>
        <code>{info.mcp_url}</code>
        <button className="connect-url-copy" onClick={copy} aria-label="Copy MCP URL">
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>

      {error && (
        <div
          className="card"
          style={{
            marginTop: "1rem",
            borderColor: "#ef4444",
            color: "#ef4444",
          }}
        >
          {error}
        </div>
      )}

      <h3 style={{ marginTop: "2rem" }}>Detected on this machine</h3>
      {autoDetected.length === 0 && (
        <p className="muted" style={{ marginTop: ".5rem" }}>
          No MCP-compatible clients found. Install one of the options below to
          get started.
        </p>
      )}
      <div className="connect-list" style={{ marginTop: ".75rem" }}>
        {autoDetected.map((c) => (
          <ClientRow
            key={c.id}
            client={c}
            busy={busy === c.id}
            justConnected={justConnected === c.id}
            onConnect={() => connect(c)}
            onDisconnect={() => disconnect(c)}
          />
        ))}
      </div>

      {manualClients.length > 0 && (
        <>
          <h3 style={{ marginTop: "2rem" }}>Manual setup</h3>
          <p
            className="muted"
            style={{ marginTop: ".25rem", fontSize: ".82rem" }}
          >
            These clients don't expose a config file — paste the MCP URL using
            their in-app settings.
          </p>
          <div className="connect-list" style={{ marginTop: ".75rem" }}>
            {manualClients.map((c) => (
              <ManualRow key={c.id} client={c} />
            ))}
          </div>
        </>
      )}

      {undetected.length > 0 && (
        <>
          <h3 style={{ marginTop: "2rem" }}>Other clients</h3>
          <p
            className="muted"
            style={{ marginTop: ".25rem", fontSize: ".82rem" }}
          >
            Install any of these to connect — WriteKit will detect them
            automatically.
          </p>
          <div className="connect-list" style={{ marginTop: ".75rem" }}>
            {undetected.map((c) => (
              <div key={c.id} className="connect-row is-unavailable">
                <div className="connect-row-icon">
                  <ClientIcon clientId={c.id} />
                </div>
                <div className="connect-row-meta">
                  <div className="connect-row-head">
                    <span className="connect-row-name">{c.name}</span>
                  </div>
                  <div className="connect-row-path">Not installed</div>
                </div>
                <span />
              </div>
            ))}
          </div>
        </>
      )}
    </>
  );
}

function ClientRow({
  client,
  busy,
  justConnected,
  onConnect,
  onDisconnect,
}: {
  client: ClientInfo;
  busy: boolean;
  justConnected: boolean;
  onConnect: () => void;
  onDisconnect: () => void;
}) {
  return (
    <div className="connect-row">
      <div className="connect-row-icon">
        <ClientIcon clientId={client.id} />
      </div>
      <div className="connect-row-meta">
        <div className="connect-row-head">
          <span className="connect-row-name">{client.name}</span>
          {client.connected && (
            <span className="connect-pill connect-pill-success">Connected</span>
          )}
          {client.requires_npx && (
            <span
              className="connect-pill connect-pill-info"
              title="This client only speaks the stdio version of MCP, so WriteKit bridges it through npx mcp-remote. Requires Node.js installed on your machine."
            >
              Needs Node
            </span>
          )}
        </div>
        <div className="connect-row-path" title={client.config_path}>
          {client.config_path}
        </div>
        {justConnected && (
          <div className="connect-row-success">
            Added. Restart {client.name} to see WriteKit's tools.
          </div>
        )}
      </div>
      {client.connected ? (
        <button
          className="btn btn-outline"
          onClick={onDisconnect}
          disabled={busy}
        >
          {busy ? "..." : "Remove"}
        </button>
      ) : (
        <button className="btn" onClick={onConnect} disabled={busy}>
          {busy ? "..." : "Connect"}
        </button>
      )}
    </div>
  );
}

function ManualRow({ client }: { client: ClientInfo }) {
  return (
    <div className="connect-row connect-row-manual">
      <div className="connect-row-icon">
        <ClientIcon clientId={client.id} />
      </div>
      <div className="connect-row-meta">
        <div className="connect-row-head">
          <span className="connect-row-name">{client.name}</span>
          <span className="connect-pill connect-pill-info">Manual</span>
        </div>
        {client.instructions && client.instructions.length > 0 && (
          <ol className="connect-row-instructions">
            {client.instructions.map((step, i) => (
              <li key={i}>{step}</li>
            ))}
          </ol>
        )}
      </div>
      <span />
    </div>
  );
}
