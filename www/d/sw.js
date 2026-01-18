self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', (event) => event.waitUntil(self.clients.claim()));

const streamMap = new Map();
let i = 0;

self.onmessage = (event) => {
    if (!event.source) {
        console.warn("Missing source client")
        return
    }

    if (event.data.type === 'REGISTER_STREAM') {
        const id = i;
        i++;
        const downloadURL = `${self.location.href}/virtual-download/${id}/${encodeURIComponent(event.data.filename)}`;
        streamMap.set(downloadURL, {stream: event.data.readableStream, length: event.data.length});

        event.source.postMessage({
            type: 'STREAM_REGISTERED',
            downloadURL: downloadURL
        });
    }
};

self.onfetch = (event) => {
    const url = event.request.url;

    if (streamMap.has(url)) {
        const entry = streamMap.get(url);
        streamMap.delete(url);
        const stream = entry.stream;
        const length = entry.length;

        // Extract filename from URL
        const filename = decodeURIComponent(url.split('/').pop());

        const headers = new Headers({
            'Content-Type': 'application/octet-stream; charset=utf-8',
            'Content-Disposition': `attachment; filename="${filename}"`,
            'X-Content-Type-Options': 'nosniff'
        });

        if (length) {
            headers.set('Content-Length', length)
        }

        event.respondWith(new Response(stream, { headers }));
    }
};