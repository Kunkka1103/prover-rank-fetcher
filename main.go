package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tealeg/xlsx/v3"
	"github.com/urfave/cli/v2"
)

type Prover struct {
	Address                      string      `json:"Address"`
	TotalPuzzleCredits           json.Number `json:"TotalPuzzleCredits"`
	TotalPuzzleCreditsPercentage string      `json:"TotalPuzzleCreditsPercentage"`
	DailyPuzzleCredits           json.Number `json:"DailyPuzzleCredits"`
	DailyPuzzleCreditsPercentage string      `json:"DailyPuzzleCreditsPercentage"`
	NetworkSpeed                 json.Number `json:"NetworkSpeed"`
	NetworkSpeedPercentage       string      `json:"NetworkSpeedPercentage"`
}

func fetchProverRankList(apiURL string, startTime, endTime int64) ([]Prover, error) {
	payload := fmt.Sprintf(`{"start_time": %d, "end_time": %d}`, startTime, endTime)
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get data: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data struct {
		Data []Prover `json:"data"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	return data.Data, nil
}

func readClusterNames(filePath string) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	clusterMap := make(map[string]string)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) < 2 {
			continue
		}
		clusterMap[record[1]] = record[0] // Assuming the first column is name and second column is address
	}

	return clusterMap, nil
}

func parseDateTime(dateTimeStr string) (int64, error) {
	const layout = "2006-01-02 15:04:05"
	local, err := time.LoadLocation("Local")
	if err != nil {
		return 0, err
	}
	t, err := time.ParseInLocation(layout, dateTimeStr, local)
	if err != nil {
		return 0, err
	}
	return t.Unix(), nil
}

func main() {
	app := &cli.App{
		Name:  "Prover Rank List Fetcher",
		Usage: "Fetch and process prover rank list",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "start_datetime",
				Usage:    "Start date and time for the data fetch (YYYY-MM-DD HH:MM:SS)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "end_datetime",
				Usage:    "End date and time for the data fetch (YYYY-MM-DD HH:MM:SS)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "cluster_file",
				Usage:    "Path to the cluster-name file",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "api_url",
				Usage:    "API URL for fetching prover rank list",
				Value:    "http://localhost:8088/api/v1/provers/prover_rank_list",
				Required: false,
			},
		},
		Action: func(c *cli.Context) error {
			startDateTime := c.String("start_datetime")
			endDateTime := c.String("end_datetime")
			clusterFilePath := c.String("cluster_file")
			apiURL := c.String("api_url")

			startTime, err := parseDateTime(startDateTime)
			if err != nil {
				return fmt.Errorf("error parsing start date and time: %v", err)
			}

			endTime, err := parseDateTime(endDateTime)
			if err != nil {
				return fmt.Errorf("error parsing end date and time: %v", err)
			}

			fmt.Printf("Start timestamp: %d\n", startTime)
			fmt.Printf("End timestamp: %d\n", endTime)

			provers, err := fetchProverRankList(apiURL, startTime, endTime)
			if err != nil {
				return fmt.Errorf("error fetching prover rank list: %v", err)
			}

			clusterNames, err := readClusterNames(clusterFilePath)
			if err != nil {
				return fmt.Errorf("error reading cluster names: %v", err)
			}

			file := xlsx.NewFile()
			sheet, err := file.AddSheet("Sheet1")
			if err != nil {
				return fmt.Errorf("error creating sheet: %v", err)
			}

			headerStyle := xlsx.NewStyle()
			headerFont := xlsx.NewFont(12, "Calibri")
			headerFont.Bold = true
			headerStyle.Font = *headerFont

			header := []string{"排名", "标记", "地址", "累计出块奖励(Puzzle Credits)", "占全网比例", "昨日奖励", "单日奖励占比", "节点速率(M s/s)", "速率占比", "GPU数量/3080", "GPU数量/4090"}
			row := sheet.AddRow()
			for _, h := range header {
				cell := row.AddCell()
				cell.Value = h
				cell.SetStyle(headerStyle)
			}

			for i, prover := range provers {
				row := sheet.AddRow()
				row.AddCell().Value = strconv.Itoa(i + 1) // 排名
				row.AddCell().Value = clusterNames[prover.Address] // 标记
				row.AddCell().Value = prover.Address // 地址

				totalPuzzleCredits, _ := prover.TotalPuzzleCredits.Float64()
				row.AddCell().SetFloatWithFormat(totalPuzzleCredits, "0.00") // 累计出块奖励(Puzzle Credits)

				row.AddCell().Value = prover.TotalPuzzleCreditsPercentage // 占全网比例

				dailyPuzzleCredits, _ := prover.DailyPuzzleCredits.Float64()
				row.AddCell().SetFloatWithFormat(dailyPuzzleCredits, "0.00") // 昨日奖励

				row.AddCell().Value = prover.DailyPuzzleCreditsPercentage // 单日奖励占比

				networkSpeed, _ := prover.NetworkSpeed.Float64()
				row.AddCell().SetFloatWithFormat(networkSpeed / 1e6, "0.00") // 节点速率(M s/s)

				row.AddCell().Value = prover.NetworkSpeedPercentage // 速率占比

				row.AddCell().SetInt(int(networkSpeed / 15000)) // GPU数量/3080
				row.AddCell().SetInt(int(networkSpeed / 43000)) // GPU数量/4090
			}

			today := time.Now().Format("2006-01-02")
			outputFileName := fmt.Sprintf("aleo大矿工统计-%s.xlsx", today)
			if err := file.Save(outputFileName); err != nil {
				return fmt.Errorf("error saving file: %v", err)
			}

			fmt.Printf("数据已保存到 %s\n", outputFileName)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
