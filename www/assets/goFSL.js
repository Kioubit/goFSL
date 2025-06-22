import "./webstreams-ponyfill.min.js";

const streamSaver = window.streamSaver;

(() => {
    if (!window.isSecureContext) {
        alert("This page works only in secure contexts (HTTPS)");
    }
})()

export async function getFileMetaData(id, key) {
    const result = await fetch("getFileMeta?id=" + encodeURIComponent(id))
    if (!result.ok) {
        const errorText = await result.text();
        if (errorText !== "") {
            throw new Error(errorText)
        }
        throw new Error("Server error")
    }
    const resultJSON = await result.json();
    const resultArray = base64ToArrayBuffer(resultJSON["UserMetaData"]);

    const rawKey = base64ToArrayBuffer(key);
    const k = await window.crypto.subtle.importKey("raw", rawKey, "AES-GCM", false, ["decrypt"]);

    resultJSON["UserMetaData"] = new TextDecoder().decode(await decryptData(resultArray, k, BigInt(0)));
    resultJSON["key"] = k;
    resultJSON["id"] = id;
    return resultJSON;
}

export async function uploadFile(file, expiry, maxDownloads, onProgress) {
    try {
        const filename = file.name;
        const fileSize = file.size;
        const stream = file.stream();

        const requestParams = new URLSearchParams();
        requestParams.set("expiry", expiry);
        requestParams.set("max_downloads", maxDownloads)
        const ws = await openWebsocket("upload?" + requestParams.toString());


        ws.onclose = (ev) => {
            if (ev.code === 4000) {
                throw new Error(ev.reason);
            }
        }

        const key = await window.crypto.subtle.generateKey({name: 'AES-GCM', length: 256}, true, ["encrypt"]);

        let chunkCounter = BigInt(0);

        // Send user metadata
        await sendOnWebsocket(ws, await encryptData(new TextEncoder().encode(filename), key, chunkCounter));
        chunkCounter += BigInt(1);

        let sentBytes = 0;
        let lastProgressPercent = "0";

        const mReader = new FullStreamReader(5000008, stream)

        while (true) {
            const {done, value} = await mReader.read()
            if (done) {
                break;
            }

            const encryptedChunk = await encryptData(value, key, chunkCounter);
            await sendOnWebsocket(ws, encryptedChunk);
            await receiveFromWebsocket(ws);
            chunkCounter += BigInt(1);

            sentBytes += value.byteLength;
            const progressPercent = ((sentBytes / fileSize) * 100).toFixed(1)
            if (progressPercent !== lastProgressPercent) {
                onProgress(progressPercent);
                lastProgressPercent = progressPercent;
            }

        }

        await sendOnWebsocket(ws, "COMPLETED")
        const result = await receiveFromWebsocket(ws)
        ws.close();

        const resultJSON = JSON.parse(result.data)

        const exportedKey = arrayBufferToBase64(await window.crypto.subtle.exportKey('raw', key));

        const dlParams = new URLSearchParams();
        dlParams.set("id", resultJSON.ID);
        dlParams.set("key", exportedKey);

        const deleteParams = new URLSearchParams();
        deleteParams.set("id", resultJSON.ID);
        deleteParams.set("deletionToken", resultJSON.DeletionToken);

        return {Download: `${window.location.href.slice(0, -2)}d/#${dlParams.toString()}`, Delete: `${window.location.href.slice(0, -2)}d/deleteFile?${deleteParams.toString()}`}
    } catch (e) {
        console.log(e)
        throw new Error(e.toString());
    }
}

class FullStreamReader {
    buffer;
    reader;
    bufferLength = 0;
    complete = false;

    constructor(bufferLength, stream) {
        this.bufferLength = bufferLength;
        this.buffer = new ArrayBuffer(bufferLength);
        this.reader = stream.getReader({'mode': 'byob'})
    }

    async read() {
        if (this.complete) {
            return {done: true, value: new Uint8Array(0)};
        }
        let offset = 0;
        while (offset < this.bufferLength) {
            const {done, value} = await this.reader.read(
                new Uint8Array(this.buffer, offset, this.bufferLength - offset)
            );
            this.buffer = value.buffer;
            if (done) {
                break;
            }
            offset += value.byteLength;
        }
        if (offset === 0) {
            return {done: true, value: new Uint8Array(0)};
        } else if (offset !== this.bufferLength) {
            this.complete = true;
            return {done: false, value: new Uint8Array(this.buffer, 0, offset)};
        } else {
            return {done: false, value: new Uint8Array(this.buffer)};
        }
    }
}

function encryptData(chunk, key, chunkCounter) {
    const dv = new DataView(new ArrayBuffer(16), 0);
    dv.setBigUint64(0, chunkCounter);
    return window.crypto.subtle.encrypt({name: 'AES-GCM', iv: dv.buffer, tagLength: 128}, key, chunk);
}

function decryptData(chunk, key, chunkCounter) {
    const dv = new DataView(new ArrayBuffer(16), 0);
    dv.setBigUint64(0, chunkCounter);
    return window.crypto.subtle.decrypt({name: 'AES-GCM', iv: dv.buffer, tagLength: 128}, key, chunk);
}


export async function downloadFile(fileMetaData, onProgress) {
    const key = fileMetaData.key;
    const id = fileMetaData.id;


    let filestream;
    if ("showSaveFilePicker" in self) {
        const handle = await showSaveFilePicker({suggestedName: fileMetaData.UserMetaData});
        filestream = await handle.createWritable();
    } else {
        streamSaver.mitm = './../assets/streamsaver/mitm.html'
        filestream = streamSaver.createWriteStream(fileMetaData.UserMetaData, {})
    }

    const writer = await filestream.getWriter();

    window.onpagehide = () => {
        writer.abort()
    }

    window.onbeforeunload = () => {
        return "Are you sure you want to leave?";
    }


    const ws = await openWebsocket("download?id=" + encodeURIComponent(id));
    let chunkCounter = BigInt(1)

    await sendOnWebsocket(ws, "chunk")

    const closePromise = new Promise((accept, reject) => {
        ws.onclose = (ev) => {
            if (ev.code === 4000) {
                reject(new Error(ev.reason));
            } else {
                accept();
            }
        }
    })

    let receivedBytes = 0;
    let lastProgressPercent = "0";

    ws.onerror = (err) => {
        console.log(err)
    }

    ws.onmessage = async (evt) => {
        try {
            const received = await evt.data.arrayBuffer();
            if (received.byteLength === 0) {
                ws.close();
            } else {
                receivedBytes += received.byteLength;
                const decryptedChunk = await decryptData(received, key, chunkCounter);
                await writer.write(new Uint8Array(decryptedChunk))
                chunkCounter += BigInt(1);

                const progressPercent = ((receivedBytes / fileMetaData.DownloadSize) * 100).toFixed(1)
                if (progressPercent !== lastProgressPercent) {
                    onProgress(progressPercent);
                    lastProgressPercent = progressPercent;
                }

                await sendOnWebsocket(ws, "chunk")
            }
        } catch (e) {
            console.log(e)
            ws.close()
        }
    }

    await closePromise;
    if (receivedBytes !== fileMetaData.DownloadSize) {
        await writer.abort()
        throw new Error("received incomplete file. Was the connection interrupted?")
    }

    await writer.close();
    window.onbeforeunload = () => {};
}

function arrayBufferToBase64(buffer) {
    let binary = '';
    const bytes = new Uint8Array(buffer);
    const len = bytes.byteLength;
    for (let i = 0; i < len; i++) {
        binary += String.fromCharCode(bytes[i]);
    }
    return window.btoa(binary);
}

function base64ToArrayBuffer(base64) {
    const binaryString = atob(base64);
    const bytes = new Uint8Array(binaryString.length);
    for (let i = 0; i < binaryString.length; i++) {
        bytes[i] = binaryString.charCodeAt(i);
    }
    return bytes.buffer;
}

async function openWebsocket(path) {
    const protocol = window.location.protocol === "https:" ? "wss://" : "ws://";
    const hostPath = window.location.host + window.location.pathname;
    const ws = new WebSocket(protocol + hostPath + path)
    const startPromise = new Promise((resolve, reject) => {
        ws.onopen = function () {
            resolve()
        }
        ws.onerror = (evt) => {
            reject(evt);
        }
    })
    await startPromise;
    return ws;
}

function receiveFromWebsocket(ws) {
    return new Promise((accept, reject) => {
        ws.onerror = (evt) => {
            reject(evt);
        }
        ws.onmessage = (evt) => {
            accept(evt);
        }
    })
}

async function sendOnWebsocket(ws, data) {
    if (ws.readyState !== WebSocket.OPEN) {
        throw new Error("Invalid websocket state")
    }
    await ws.send(data)
}