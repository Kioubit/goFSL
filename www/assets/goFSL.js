//import "./webstreams-ponyfill.min.js";

(() => {
    if (!window.isSecureContext) {
        alert("This page only works in secure contexts (HTTPS)");
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

    resultJSON["UserMetaData"] = JSON.parse(new TextDecoder().decode(await decryptData(resultArray, k, BigInt(0))));
    resultJSON["key"] = k;
    resultJSON["id"] = id;
    return resultJSON;
}

export async function uploadFile(file, expiry, maxDownloads, onProgress) {
    const BUFFER_THRESHOLD = 3 * 1024 * 1024; // 3MB

    return new Promise(async (resolve, reject) => {
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
                    reject(new Error(ev.reason))
                }
            }

            const key = await window.crypto.subtle.generateKey({name: 'AES-GCM', length: 256}, true, ["encrypt"]);

            let chunkCounter = BigInt(1); // Zero is reserved for metadata

            let sentBytesPlain = 0;
            let sentBytesEncrypted = 0;
            let lastProgressPercent = "0";

            const mReader = new FullStreamReader(3000000, stream)

            while (true) {
                const {done, value} = await mReader.read()
                if (done) {
                    break;
                }

                const encryptedChunk = await encryptData(value, key, chunkCounter);
                chunkCounter += BigInt(1);
                await sendOnWebsocket(ws, encryptedChunk);
                sentBytesPlain += value.byteLength;
                sentBytesEncrypted += encryptedChunk.byteLength;

                while (ws.bufferedAmount > BUFFER_THRESHOLD) {
                    await new Promise(r => setTimeout(r, 10));
                }

                const progressPercent = ((sentBytesPlain / fileSize) * 100).toFixed(1)
                if (progressPercent !== lastProgressPercent) {
                    onProgress(progressPercent);
                    lastProgressPercent = progressPercent;
                }
            }
            // Edge case for zero byte files
            onProgress("100");

            // Send amount of sent encrypted bytes
            await sendOnWebsocket(ws, sentBytesEncrypted.toString(10))

            // Send user metadata
            const userMetadata = {
                "filename": filename,
                "plainBytes": sentBytesPlain,
            };
            await sendOnWebsocket(ws, await encryptData(new TextEncoder().encode(JSON.stringify(userMetadata)), key, BigInt(0)));

            // Receive upload details
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

            resolve({
                "Download": `${window.location.href.slice(0, -2)}d/#${dlParams.toString()}`,
                "Delete": `${window.location.href.slice(0, -2)}d/deleteFile?${deleteParams.toString()}`
            });
        } catch (err) {
            console.log(err)
            reject(err);
        }
    });
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
    const dv = new DataView(new ArrayBuffer(12), 0);
    dv.setBigUint64(0, chunkCounter);
    return window.crypto.subtle.encrypt({name: 'AES-GCM', iv: dv.buffer, tagLength: 128}, key, chunk);
}

function decryptData(chunk, key, chunkCounter) {
    const dv = new DataView(new ArrayBuffer(12), 0);
    dv.setBigUint64(0, chunkCounter);
    return window.crypto.subtle.decrypt({name: 'AES-GCM', iv: dv.buffer, tagLength: 128}, key, chunk);
}

// Approach via service worker offers better UX
const ENABLE_SAVE_FILE_PICKER = false;

export async function downloadFile(fileMetaData, onProgress) {
    const filename = fileMetaData.UserMetaData.filename;
    const downloadSize = fileMetaData.DownloadSize;
    const decryptedSize = fileMetaData.UserMetaData.plainBytes;
    const key = fileMetaData.key;
    const id = fileMetaData.id;

    let writer;
    if (ENABLE_SAVE_FILE_PICKER && "showSaveFilePicker" in self) {
        const handle = await showSaveFilePicker({suggestedName: filename});
        const filestream = await handle.createWritable();
        writer = await filestream.getWriter();
    } else {
        const { readable, writable } = new TransformStream();
        writer = writable.getWriter();
        await streamToServiceWorker(readable, filename, decryptedSize);
    }

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
                writer.abort();
                reject(new Error(ev.reason));
            } else {
                accept();
            }
        }
    })

    let receivedBytes = 0;
    let lastProgressPercent = "0";

    ws.onerror = (err) => {
        console.log(err);
        writer.abort();
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

                const progressPercent = ((receivedBytes / downloadSize) * 100).toFixed(1)
                if (progressPercent !== lastProgressPercent) {
                    onProgress(progressPercent);
                    lastProgressPercent = progressPercent;
                }

                // Request the next chunk manually as we cannot apply backpressure with this websocket API
                await sendOnWebsocket(ws, "chunk")
            }
        } catch (e) {
            console.log(e)
            ws.close()
        }
    }

    await closePromise;
    if (receivedBytes !== downloadSize) {
        console.log("Received bytes", receivedBytes, "Expected bytes", downloadSize)
        await writer.abort()
        throw new Error("Download was interrupted")
    }

    await writer.close();
    window.onbeforeunload = () => {};
}

async function streamToServiceWorker(stream, filename, decryptedSize) {
    if (!navigator.serviceWorker.controller) {
        await navigator.serviceWorker.register('sw.js',{ scope: './' });
        await navigator.serviceWorker.ready;
        if (!navigator.serviceWorker.controller) {
            // Wait for controllerchange or timeout
            await Promise.race([
                new Promise(resolve =>
                    navigator.serviceWorker.addEventListener('controllerchange', resolve, { once: true })
                ),
                new Promise(resolve => setTimeout(resolve, 1000))
            ]);

            if (!navigator.serviceWorker.controller) {
                return window.location.reload();
            }
        }
    }

    const registrationPromise = new Promise((resolve, reject) => {
        const timeout = setTimeout(() => {
            navigator.serviceWorker.removeEventListener('message', onMessage);
            reject(new Error("Service Worker timed out waiting for stream registration"));
        }, 5000);

        const onMessage = (event) => {
            if (event.data.type === 'STREAM_REGISTERED') {
                clearTimeout(timeout);
                navigator.serviceWorker.removeEventListener('message', onMessage);
                resolve(event.data.downloadURL);
            }
        };

        navigator.serviceWorker.addEventListener('message', onMessage);
    });

    // Send the ReadableStream to the Service Worker
    // The stream is moved out of the main thread
    navigator.serviceWorker.controller.postMessage({
        type: 'REGISTER_STREAM',
        filename: filename,
        readableStream: stream,
        length: decryptedSize,
    }, [stream]);


    const downloadUrl = await registrationPromise;

    // Trigger the download via Navigation
    // Since the SW returns "Content-Disposition: attachment",
    // the page will not change; it will just pop up the save dialog.
    window.location.assign(downloadUrl);
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
            console.log(evt);
            reject(new Error("Websocket connection failed"));
        }
    })
    await startPromise;
    return ws;
}

function receiveFromWebsocket(ws) {
    return new Promise((accept, reject) => {
        ws.onerror = (evt) => {
            console.log(evt)
            reject(new Error("Websocket error"));
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