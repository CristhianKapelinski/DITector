<template>
    <div class="input-with-search">
        <el-input id="input-1" v-model="searchKeyword" placeholder="image digest" clearable>
            <template #append>
                <el-button id="search-1" type="primary" @click="handleSearchImages">Search</el-button>
            </template>
        </el-input>
    </div>
    <el-table
            :data="imagesData"
            highlight-current-row
            stripe
            table-layout="fixed"
            style="width: 100%"
            max-height="700"
    >
<!--        可收缩展开内容-->
        <el-table-column fixed type="expand">
            <template #default="props">
                <div>
                  <el-table
                          id="expanded-table"
                          highlight-current-row
                          :data="props.row.layers"
                          :row-class-name="tableRowClassName"
                  >
<!--                      used for left white-->
                      <el-table-column label="" width="50" />
                      <el-table-column prop="colId" label="Index" align="center" width="80" />
                      <el-table-column prop="instruction" label="Instruction" width="450" />
                      <el-table-column prop="size" label="Size" align="center" width="125" />
                      <el-table-column prop="digest" label="Digest" width="650" />
                      <el-table-column prop="results" label="Results" />
                  </el-table>
                </div>
            </template>
        </el-table-column>
        <el-table-column fixed prop="digest" label="Digest" show-overflow-tooltip width="650" />
        <el-table-column prop="architecture" label="Architecture" show-overflow-tooltip align="center" width="120" />
        <el-table-column prop="features" label="Features" show-overflow-tooltip align="center" width="100" />
        <el-table-column prop="variant" label="Variant" show-overflow-tooltip align="center" width="100" />
        <el-table-column prop="os" label="OS" show-overflow-tooltip align="center" width="100" />
        <el-table-column prop="size" label="Size" align="center" width="125" />
        <el-table-column prop="status" label="Status" align="center" width="100" />
        <el-table-column prop="last_pulled" label="Last Pulled" align="center" width="240" />
        <el-table-column prop="last_pushed" label="Last Pushed" align="center" width="240" />
    </el-table>
    <div class="pagination-bottom">
      <el-pagination
              :currentPage="currentPage"
              :page-sizes="[10, 20, 50]"
              :page-size="pageSize"
              layout=" prev, pager, next, jumper, sizes, total, "
              :total="totalPages"
              @size-change="handleSizeChange"
              @current-change="handleCurrentChange"
              align="center"
      />
    </div>
</template>

<script lang="ts" setup>
import { ref } from 'vue';
import axios from 'axios';

// row-class-name 对每行内容处理的回调函数
const tableRowClassName = ({row, rowIndex}) => {
    row.colId = rowIndex + 1;
};

const currentPage = ref(1);
const pageSize = ref(20);
const totalCnt = ref(0);    // total count of documents in response
const totalPages = ref(0);  // total count of pages (totalCnt/pageSize + 1)
const searchKeyword = ref('');
const imagesData = ref([]);

function handleSearchImages() {
  // console.log("button clicked");
  // reset to page 1 before every search
  currentPage.value = 1;
  fetchImagesData();
}

function getImagesData(search, currentPage, pageSize) {
  // axios get images data responsed from backend API
  axios.get('http://10.10.21.122:23434/images', {
      params: {
          search: search,
          page: currentPage,
          page_size: pageSize
      }
  }).then(response => {
      imagesData.value = response.data['results'];
      totalCnt.value = response.data['count'];
      // console.log(imagesData.value);
      // console.log(response.data);
      recalculateTotalPages();
  })
  .catch(error => {
      console.log(error);
  });
}

function handleCurrentChange(val: number) {
    currentPage.value = val;
    console.log(currentPage.value);
    fetchImagesData();
}

function handleSizeChange(val: number) {
    currentPage.value = 1;
    // change pageSize
    pageSize.value = val;
    // recalculate totalPages
    recalculateTotalPages();
    console.log(pageSize.value);
    fetchImagesData();
}

// recalculate TotalPages after totalCnt or pageSize changed
function recalculateTotalPages() {
    if (totalCnt.value === 0) {
        totalPages.value = 0;
    } else {
        totalPages.value = Math.floor(totalCnt.value / pageSize.value + 1);
    }
}

// fetch images data from backend with searchKeyword, currentPage and pageSize
function fetchImagesData() {
    getImagesData(searchKeyword.value, currentPage.value, pageSize.value);
}

// init web page
fetchImagesData();
</script>

<style scoped>
.input-with-search {
    float: right;
    width: 45%;
}

.pagination-bottom {
    margin-top: 20;
    display: flex;
    justify-content: center;
}

</style>