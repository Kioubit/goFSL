import * as goFSL from "../assets/goFSL.js";

const downloadBtn = document.getElementById("downloadBtn");
const progressBar = document.getElementById("progressBar");
const fileNameDiv = document.getElementById("fileName");
const errorDiv = document.getElementById("errorDisplay");
const metadataDiv = document.getElementById("metadata");

let fileMeta = null;

const urlParams = new URLSearchParams(window.location.hash.substring(1));
const fileID = urlParams.get("id");
const fileKey = urlParams.get("key");
if (fileKey !== null && fileID !== null) {
    goFSL.getFileMetaData(urlParams.get("id"), urlParams.get("key")).then((r)=> {
        fileMeta = r;
        fileNameDiv.textContent = fileMeta.UserMetaData.filename;
        downloadBtn.disabled = false;
        progressBar.value = 0;
        metadataDiv.innerText = `Download size: ${bytesToHuman(fileMeta.DownloadSize)}
            Downloads remaining: ${fileMeta.DownloadsRemaining > -1 ? fileMeta.DownloadsRemaining : 'unlimited'}
            Expiry: ${new Date(fileMeta.Expiry*1000).toLocaleString()}`;
        metadataDiv.classList.remove("noDisplay");
    }).catch(err => {
        fileNameDiv.innerText = "";
        errorDiv.innerText = err.toString();
        progressBar.value = 0;
    })
} else {
    fileNameDiv.innerText = "";
    errorDiv.innerText = "Invalid URL";
    progressBar.value = 0;
}

downloadBtn.onclick = () => {
    downloadBtn.disabled = true;
    goFSL.downloadFile(fileMeta, (progress) => {
        downloadBtn.innerText = "Downloading ...";
        progressBar.value = progress/100;
    }).then(() => {
        downloadBtn.innerText = "Downloaded";
    }).catch(err => {
        if (err.name === 'AbortError') {
            downloadBtn.disabled = false;
            return;
        }
        fileNameDiv.innerText = "";
        downloadBtn.innerText = "Error";
        errorDiv.innerText = err.toString();
    })
}

function bytesToHuman(value) {
    if (typeof value !== "number") {
        value = parseInt(value);
    }
    const units = ["b", "kb", "mb", "gb", "tb", "pb"];
    let i = 0;
    while (true) {
        if (i >= units.length) {
            i--
            break
        }
        if (value < 1000) {
            break
        }
        value = value / 1000
        i++;
    }
    if (i !== 0) {
        value = value.toFixed(2)
    }
    return value + " " + units[i];
}