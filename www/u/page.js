import * as goFSL from "../assets/goFSL.js";

const form = document.getElementById("dataInput");
const dropZone = document.getElementById("dropZone");
const fileInput = document.getElementById("fileInput");
const progressBar = document.getElementById("progress");
const resultDiv = document.getElementById("resultDiv");
const downloadLink = document.getElementById("downloadLink");
const deletionLink = document.getElementById("deletionLink");

dropZone.addEventListener("click", () => {
    fileInput.click();
});
dropZone.addEventListener("drop", dropHandler)
dropZone.addEventListener("dragover", (ev) => {
    ev.preventDefault();
    dropZone.classList.add("active");
})
dropZone.addEventListener("dragleave", (ev) => {
    ev.preventDefault();
    dropZone.classList.remove("active");
})

function dropHandler(ev) {
    ev.preventDefault();
    fileInput.files = ev.dataTransfer.files;
    handleFileInput()
    dropZone.classList.remove("active");
}

fileInput.onchange = async () => {
    handleFileInput()
}

form.onchange = () => {
    form.elements.maxDownloads.disabled = form.elements.unlimitedDownloads.checked;
};

function handleFileInput() {
    const files = fileInput.files;
    if (files.count === 0) {
        alert("No file selected");
        return;
    }
    const file = files[0];
    if (file.name === undefined) {
        return;
    }

    downloadLink.innerText = "";
    downloadLink.href = "#";
    deletionLink.innerText = "";
    deletionLink.href = "#";
    form.classList.add("disable");
    form.inert = true;
    progressBar.removeAttribute("value");
    resultDiv.classList.add("noDisplay");

    const formData = new FormData(form);
    let maxDownloads = formData.get("maxDownloads");
    if (formData.get("unlimitedDownloads")) {
        maxDownloads = -1;
    }

    goFSL.uploadFile(file, Math.trunc(new Date() / 1000) + parseInt(formData.get("expiry"), 10), maxDownloads, function (progressPercent) {
        progressBar.value = progressPercent / 100;
    }).then((result) => {
        resultDiv.classList.remove("noDisplay");
        downloadLink.innerText = result.Download;
        downloadLink.href = result.Download;
        deletionLink.innerText = result.Delete;
        deletionLink.href = result.Delete;
        alert("Upload completed")
    }).catch(err => {
        console.log(err)
        alert(err)
    }).finally(() => {
        form.inert = false;
        form.classList.remove("disable");
        form.reset();
    });
}