// Tiny HS256/HS384/HS512 JWT signer. We avoid pulling in a 300 kB `jsonwebtoken`
// dep — the browser's WebCrypto already does HMAC for us. The signer is
// invoked just before send when Auth.type === 'jwt'.

function b64url(bytes: ArrayBuffer | Uint8Array): string {
  const u8 = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes)
  let s = ''
  for (const b of u8) s += String.fromCharCode(b)
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}
function strToBytes(s: string): Uint8Array {
  return new TextEncoder().encode(s)
}

export async function signJwt(
  algo: 'HS256' | 'HS384' | 'HS512',
  secret: string,
  claimsJson: string,
): Promise<string> {
  const hashName = algo === 'HS256' ? 'SHA-256' : algo === 'HS384' ? 'SHA-384' : 'SHA-512'
  const header = { alg: algo, typ: 'JWT' }

  let payload: any = {}
  if (claimsJson.trim()) {
    try { payload = JSON.parse(claimsJson) } catch { payload = {} }
  }
  // Auto-stamp `iat` if the user hasn't supplied one — saves a chunk of
  // friction for the most common "just gimme a token" case.
  if (payload.iat === undefined) payload.iat = Math.floor(Date.now() / 1000)

  const head = b64url(strToBytes(JSON.stringify(header)))
  const body = b64url(strToBytes(JSON.stringify(payload)))
  const signingInput = `${head}.${body}`

  const key = await crypto.subtle.importKey(
    'raw', strToBytes(secret),
    { name: 'HMAC', hash: { name: hashName } },
    false, ['sign'],
  )
  const sig = await crypto.subtle.sign('HMAC', key, strToBytes(signingInput))
  return `${signingInput}.${b64url(sig)}`
}
