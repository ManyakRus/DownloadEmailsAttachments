set FileName=DownloadEmailsAttachments

go build main.go
del /Q %FileName%.exe
del /Q %FileName%.zip
ren main.exe %FileName%.exe
copy %FileName%.exe DownloadEmailsAttachments_ready\%FileName%.exe
copy readme.txt DownloadEmailsAttachments_ready\readme.txt
copy readme.md DownloadEmailsAttachments_ready\readme.md

"C:\Program Files\7-Zip\7z.exe" a -tzip DownloadEmailsAttachments_ready 